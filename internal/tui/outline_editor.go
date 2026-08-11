package tui

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The outline editor is the bulk-restructuring escape hatch: one project's task
// tree is dumped to $EDITOR, edited as text, and applied as a single
// transaction with one undo entry.
//
// Scope is defined by the project, not by the viewport. Dumping a complete model
// subtree is what makes indentation safe to reinterpret as parentage: there is
// no folded or filtered child off screen to orphan.

// outlineEditorModeRefusal is what a fixed-order mode answers `O` with. Today,
// Next, Due, and Recent promote rows across section boundaries, so indentation
// there is not parentage and reading it as parentage would corrupt hierarchy.
const outlineEditorModeRefusal = "Outline edit needs All, Inbox, or Backlog"

// outlineBufferPrefix and outlineBufferSuffix name the temp file the round trip
// writes. The suffix gives editors a Markdown mode, which is close enough to the
// buffer's shape to get useful highlighting.
const (
	outlineBufferPrefix = "tt-outline-"
	outlineBufferSuffix = ".md"
)

// cursorProject resolves the project the Outliner cursor sits inside, whatever
// row kind it rests on. Task rows and project headers both name their project
// directly; a scoped Outliner has only one project to name.
func (m *OutlinerModel) cursorProject() *models.Project {
	if m == nil {
		return nil
	}
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		if projectID := m.rows[m.cursor].ProjectID; projectID != "" {
			if m.project != nil && m.project.ID == projectID {
				return m.project
			}
			if project := m.crossResolveProject(projectID); project != nil {
				return project
			}
		}
	}
	return m.project
}

// outlineEditorRequest resolves what `O` would act on, or the refusal that stops
// it. An empty refusal with a nil project means there is simply nothing under
// the cursor to outline.
func (m *AppModel) outlineEditorRequest() (*models.Project, string, string) {
	out := m.activeOutliner()
	if out == nil {
		return nil, "", ""
	}
	mode, ok := OutlinerModeByID(out.primaryMode)
	if !ok || mode.FixedOrder() {
		return nil, "", outlineEditorModeRefusal
	}
	return out.cursorProject(), mode.Label, ""
}

// dumpOutlineBuffer renders a project's complete task tree.
//
// The walk follows project.Tasks order rather than the built child lists,
// because that is the order the application layer reads sibling sequence back
// from; a dump that reordered siblings would report a change nobody made.
func dumpOutlineBuffer(project *models.Project, modeLabel string, now time.Time) (string, error) {
	if project == nil {
		return "", fmt.Errorf("no project to outline")
	}
	if err := assertOutlineMarkersUnique(project); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(outlineBufferHeader(project.Name, modeLabel, now))

	children := make(map[string][]*models.Task, len(project.Tasks))
	var roots []*models.Task
	for _, task := range project.Tasks {
		switch {
		case task == nil:
		case task.ParentID == nil:
			roots = append(roots, task)
		default:
			children[*task.ParentID] = append(children[*task.ParentID], task)
		}
	}

	var walk func(task *models.Task, depth int)
	walk = func(task *models.Task, depth int) {
		builder.WriteString(outlineDumpLine(task, depth))
		builder.WriteString("\n")
		for _, child := range children[task.ID] {
			walk(child, depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}
	return builder.String(), nil
}

// outlineBufferHeader is the comment block every dump opens with. It states the
// two rules a text buffer cannot show on its own: indentation is parentage, and
// a line is a complete declaration rather than a patch.
func outlineBufferHeader(projectName, modeLabel string, now time.Time) string {
	scope := "project: " + projectName
	if modeLabel != "" {
		scope += " · mode: " + modeLabel
	}
	return strings.Join([]string{
		fmt.Sprintf("# tt outline · %s · %s", scope, now.Format("2006-01-02 15:04")),
		fmt.Sprintf("# Indent = parentage (%d spaces). Sigils: !1-3 #tag @date $destination", outlineIndentWidth),
		"# Lines are declarative: an omitted sigil clears that value.",
		fmt.Sprintf("# Keep %sxxxxxxxx%s to edit a task. No marker = new task.", outlineMarkerPrefix, outlineMarkerSuffix),
		"# Removing a line changes nothing. Delete with Space > Delete.",
		"",
		"",
	}, "\n")
}

// outlineDumpLine renders one task as its complete current value, so an omitted
// sigil on the way back in is a deliberate clear rather than an absent
// instruction.
func outlineDumpLine(task *models.Task, depth int) string {
	segments := []string{task.Title}
	if task.Priority != models.PriorityUnset {
		segments = append(segments, fmt.Sprintf("!%d", task.Priority))
	}
	if task.DueDate != nil {
		segments = append(segments, "@"+task.DueDate.Format("2006-01-02"))
	}
	for _, tag := range task.Tags {
		segments = append(segments, "#"+tag)
	}
	// The lifecycle state is dumped, but Today and Next membership is not: an
	// omitted $today cannot remove a membership (the application layer preserves
	// planning on omission), so printing one would promise a symmetry the buffer
	// does not have. Planning survives the round trip untouched.
	segments = append(segments, "$"+string(task.State()))

	line := strings.Repeat(" ", depth*outlineIndentWidth) + strings.Join(segments, " ")
	return line + "  " + outlineMarkerPrefix + outlineMarkerFor(task.ID) + outlineMarkerSuffix
}

// assertOutlineMarkersUnique refuses to open the editor over a buffer whose
// short markers collide. A collision would make two lines name the same task,
// and discovering that after the user has edited for ten minutes is worse than
// never opening.
func assertOutlineMarkersUnique(project *models.Project) error {
	seen := make(map[string]string, len(project.Tasks))
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		marker := outlineMarkerFor(task.ID)
		if other, clash := seen[marker]; clash {
			return fmt.Errorf("tasks %s and %s share the marker %s%s%s", other, task.ID, outlineMarkerPrefix, marker, outlineMarkerSuffix)
		}
		seen[marker] = task.ID
	}
	return nil
}

// outlineDumpSource renders the buffer to edit from one application snapshot.
//
// The cursor only names which project to outline. The tree that reaches the
// editor and the revision the apply is bound to are read together, so a cursor
// holding a project the shell loaded minutes ago cannot dump a stale tree under
// a current revision.
func (m *AppModel) outlineDumpSource(projectID, modeLabel string) (*models.Project, string, string, error) {
	project, revision, err := m.features.Outline.Project(projectID)
	if err != nil {
		return nil, "", "", err
	}
	buffer, err := dumpOutlineBuffer(project, modeLabel, time.Now())
	if err != nil {
		return nil, "", "", err
	}
	return project, revision, buffer, nil
}

// handleOpenOutlineEditor dumps the project and hands the terminal to $EDITOR.
func (m *AppModel) handleOpenOutlineEditor(msg OpenOutlineEditorMsg) tea.Cmd {
	if msg.Project == nil {
		return errorCmd("no project to outline")
	}
	modeLabel := ""
	if out := m.activeOutliner(); out != nil {
		if mode, ok := OutlinerModeByID(out.primaryMode); ok {
			modeLabel = mode.Label
		}
	}
	project, revision, buffer, err := m.outlineDumpSource(msg.Project.ID, modeLabel)
	if err != nil {
		return errorCmd("%s", err)
	}
	path, err := external.CreateTempFileWithContent(buffer, outlineBufferPrefix, outlineBufferSuffix)
	if err != nil {
		return errorCmd("%s", err)
	}
	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		removeOutlineBuffer(path)
		return errorCmd("%s", editorErr)
	}
	return tea.ExecProcess(external.EditorCommand(editor, path), func(execErr error) tea.Msg {
		edited := OutlineEditedMsg{Project: project, Revision: revision, Path: path}
		if execErr != nil {
			edited.Err = execErr
			return edited
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // Path is the temp file this command just wrote.
		if readErr != nil {
			edited.Err = readErr
			return edited
		}
		edited.Buffer = string(content)
		return edited
	})
}

// handleOutlineEdited validates and applies one returned buffer.
//
// The temp file survives every outcome the user might still want to act on — a
// stale workspace, a rejected buffer — and is removed only once the edit has
// either committed or become unusable.
func (m *AppModel) handleOutlineEdited(msg OutlineEditedMsg) tea.Cmd {
	if msg.Err != nil {
		removeOutlineBuffer(msg.Path)
		return errorCmd("%s", msg.Err)
	}
	if msg.Project == nil {
		removeOutlineBuffer(msg.Path)
		return errorCmd("no project to outline")
	}

	lines, lineErrors := parseOutlineBuffer(msg.Buffer, msg.Project)
	if len(lineErrors) > 0 {
		m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, lineErrors))
		return nil
	}

	// The dump revision travels with the request rather than being re-read here:
	// a check on this side of the call could only ever be a check-then-apply, and
	// the apply is where the tasks are actually mutated.
	result, err := m.features.Outline.Apply(app.OutlinePlanRequest{
		ProjectID:        msg.Project.ID,
		ExpectedRevision: msg.Revision,
		ScopeTaskIDs:     outlineScopeTaskIDs(msg.Project),
		Lines:            lines,
	})
	if errors.Is(err, storage.ErrStaleRevision) {
		m.setWarningStatus("Workspace changed while the editor was open · buffer kept at " + msg.Path)
		return defaultStatusTimeout()
	}
	if err != nil {
		m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, []lineError{outlineApplyError(err)}))
		return nil
	}

	removeOutlineBuffer(msg.Path)
	return tea.Batch(
		m.setRecoveryFeedback(result.Summary(), outlineTransactionID(result), false),
		func() tea.Msg { return SaveCompleteMsg{} },
	)
}

// outlineScopeTaskIDs names every task the buffer was dumped from, so a marked
// task the user removed from the buffer is reported as unchanged instead of
// being read as a deletion.
func outlineScopeTaskIDs(project *models.Project) []string {
	ids := make([]string, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil && task.ID != "" {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

// outlineTransactionID reads the committed transaction off any result the
// commit produced. Created, updated, and unchanged results all carry the same
// one, because the whole buffer is a single transaction.
func outlineTransactionID(result app.OutlinePlanResult) string {
	for _, group := range [][]app.TaskResult{result.Created, result.Updated, result.Unchanged} {
		for _, task := range group {
			if task.TransactionID != "" {
				return task.TransactionID
			}
		}
	}
	return ""
}

// outlineApplyError re-renders an application-layer rejection into the same
// line-numbered list the parse phases use. The application layer prefixes its
// own rejections with "line N: ", which is exactly the part the list column
// already shows.
func outlineApplyError(err error) lineError {
	message := err.Error()
	tail, prefixed := strings.CutPrefix(message, "line ")
	if !prefixed {
		return lineError{line: 1, message: message}
	}
	number, detail, split := strings.Cut(tail, ": ")
	parsed, convErr := strconv.Atoi(number)
	if !split || convErr != nil || parsed <= 0 {
		return lineError{line: 1, message: message}
	}
	return lineError{line: parsed, message: detail}
}

// removeOutlineBuffer discards a temp file the user no longer needs. A failed
// removal is not worth a status: the file is in the system temp directory and
// the edit itself already succeeded or failed on its own terms.
func removeOutlineBuffer(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
