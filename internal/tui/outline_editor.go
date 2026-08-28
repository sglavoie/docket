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
	"github.com/sglavoie/tt/internal/textgrammar"
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

// dumpOutlineBuffer renders a project's complete task tree and names every task
// it printed.
//
// The walk follows project.Tasks order rather than the built child lists,
// because that is the order the application layer reads sibling sequence back
// from; a dump that reordered siblings would report a change nobody made.
//
// The printed IDs are returned from the same walk rather than read off
// project.Tasks, because absence is a deletion now. A task whose ParentID names
// a task that no longer exists is neither a root nor anyone's child, so it never
// reaches the buffer — and a scope built from project.Tasks would let a line the
// user never saw be deleted by a line they never removed.
// projects is the loaded workspace population the dumped dependencies are
// addressed against. It is passed in rather than re-read here so the aliases the
// buffer prints and the ones the returned buffer resolves come from one view.
func dumpOutlineBuffer(project *models.Project, projects []*models.Project, modeLabel string, now time.Time) (string, []string, error) {
	if project == nil {
		return "", nil, fmt.Errorf("no project to outline")
	}
	if err := assertOutlineMarkersUnique(project); err != nil {
		return "", nil, err
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

	dumped := make([]string, 0, len(project.Tasks))
	var walk func(task *models.Task, depth int)
	walk = func(task *models.Task, depth int) {
		builder.WriteString(outlineDumpLine(task, depth, project, projects))
		builder.WriteString("\n")
		dumped = append(dumped, task.ID)
		for _, child := range children[task.ID] {
			walk(child, depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}
	return builder.String(), dumped, nil
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
		fmt.Sprintf("# Indent = parentage (%d spaces). Sigils: %s", outlineIndentWidth, outlineGrammarForms()),
		"# Lines are declarative: an omitted sigil clears that value.",
		fmt.Sprintf("# Keep %sxxxxxxxx%s to edit a task. No marker = new task.", outlineMarkerPrefix, outlineMarkerSuffix),
		"# Removing a line deletes that task; you confirm the list on return.",
		"",
		"",
	}, "\n")
}

func outlineGrammarForms() string {
	var forms []string
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceOutline) {
		forms = append(forms, definition.Forms...)
	}
	return strings.Join(forms, " ")
}

// outlineDumpLine renders one task as its complete current value, so an omitted
// sigil on the way back in is a deliberate clear rather than an absent
// instruction.
func outlineDumpLine(task *models.Task, depth int, scope *models.Project, projects []*models.Project) string {
	segments := []string{task.Title}
	priorityDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldPriority)
	dueDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDue)
	tagDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldTag)
	contextDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldContext)
	destinationDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDestination)
	pinnedDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldPinned)
	dependencyDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDependency)
	if task.Priority != models.PriorityUnset {
		segments = append(segments, fmt.Sprintf("%s%d", priorityDefinition.Trigger, task.Priority))
	}
	if task.DueDate != nil {
		segments = append(segments, dueDefinition.Trigger+dueGrammarValue(task))
	}
	for _, tag := range task.Tags {
		segments = append(segments, tagDefinition.Trigger+tag)
	}
	if task.Context != "" {
		segments = append(segments, contextDefinition.Trigger+task.Context)
	}
	// Every stored reference is printed, closed and unloaded targets included.
	// The line is a complete declaration, so a reference the dump withheld would
	// be silently deleted by a user who never saw it.
	for _, reference := range task.Dependencies {
		segments = append(segments, dependencyDefinition.Trigger+outlineDependencyAlias(reference, scope, projects))
	}
	// The lifecycle state is dumped, but Today and Next membership is not: an
	// omitted $today cannot remove a membership (the application layer preserves
	// planning on omission), so printing one would promise a symmetry the buffer
	// does not have. Planning survives the round trip untouched.
	segments = append(segments, destinationDefinition.Trigger+string(task.State()))
	if task.Pinned {
		segments = append(segments, pinnedDefinition.Trigger+"pin")
	}

	line := strings.Repeat(" ", depth*outlineIndentWidth) + strings.Join(segments, " ")
	return line + "  " + outlineMarkerPrefix + outlineMarkerFor(task.ID) + outlineMarkerSuffix
}

// outlineDependencyAlias renders one stored reference as the token the buffer
// prints. A target in the dumped project reads as a bare short ID, a target in
// another loaded project as its slug-qualified short ID, and anything this
// workspace cannot address falls back to the durable project-id:task-id locator
// — which is exactly the form the parser accepts back unchanged, so a reference
// whose target is archived or deleted survives the round trip.
func outlineDependencyAlias(reference models.TaskReference, scope *models.Project, projects []*models.Project) string {
	target := scope
	if target == nil || target.ID != reference.ProjectID {
		target = projectByStableID(reference.ProjectID, projects)
	}
	if target != nil {
		if task := target.GetTaskByID(reference.TaskID); task != nil {
			return dependencyAlias(target, task, scope)
		}
	}
	return reference.Locator()
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
func (m *AppModel) outlineDumpSource(projectID, modeLabel string) (*models.Project, string, string, []string, error) {
	project, revision, err := m.features.Outline.Project(projectID)
	if err != nil {
		return nil, "", "", nil, err
	}
	buffer, scopeTaskIDs, err := dumpOutlineBuffer(project, m.outlineDependencyProjects(project), modeLabel, time.Now())
	if err != nil {
		return nil, "", "", nil, err
	}
	return project, revision, buffer, scopeTaskIDs, nil
}

// outlineDependencyProjects is the population a dumped or parsed dependency is
// addressed against: the same non-archived list the inline edit surface resolves
// against, with the scope project substituted for the shell's copy of it.
//
// The substitution matters because the outline round trip loads its project from
// its own snapshot. Without it the buffer would resolve a same-project reference
// against a second, older tree, and a task the dump printed could fail to
// resolve on the way back in.
func (m *AppModel) outlineDependencyProjects(scope *models.Project) []*models.Project {
	available := m.availableProjects()
	projects := make([]*models.Project, 0, len(available)+1)
	replaced := false
	for _, project := range available {
		if scope != nil && project != nil && project.ID == scope.ID {
			projects = append(projects, scope)
			replaced = true
			continue
		}
		projects = append(projects, project)
	}
	if scope != nil && !replaced {
		projects = append(projects, scope)
	}
	return projects
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
	project, revision, buffer, scopeTaskIDs, err := m.outlineDumpSource(msg.Project.ID, modeLabel)
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
		edited := OutlineEditedMsg{Project: project, Revision: revision, ScopeTaskIDs: scopeTaskIDs, Path: path}
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

// handleOutlineEdited validates one returned buffer and either applies it or
// asks about the tasks it removed.
//
// The temp file survives every outcome the user might still want to act on — a
// stale workspace, a rejected buffer, a declined deletion — and is removed only
// once the edit has either committed or become unusable.
func (m *AppModel) handleOutlineEdited(msg OutlineEditedMsg) tea.Cmd {
	if msg.Err != nil {
		removeOutlineBuffer(msg.Path)
		return errorCmd("%s", msg.Err)
	}
	if msg.Project == nil {
		removeOutlineBuffer(msg.Path)
		return errorCmd("no project to outline")
	}

	lines, lineErrors := parseOutlineBuffer(msg.Buffer, msg.Project, m.outlineDependencyProjects(msg.Project))
	if len(lineErrors) > 0 {
		m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, lineErrors))
		return nil
	}

	pending := outlineApplyPending{
		project:      msg.Project,
		revision:     msg.Revision,
		path:         msg.Path,
		scopeTaskIDs: msg.ScopeTaskIDs,
		lines:        lines,
		deletions:    outlineDeletedTasks(msg.Project, msg.ScopeTaskIDs, lines),
	}
	// Emptiness is ruled on here rather than in the parser, because only this side
	// knows the scope. An emptied buffer over a scope that held tasks deletes them
	// all; over a scope that held none it deleted nothing, so it is the truncation
	// accident the parser used to assume.
	if len(lines) == 0 && len(pending.deletions) == 0 {
		m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, []lineError{{line: 1, message: "the buffer holds no tasks"}}))
		return nil
	}
	if len(pending.deletions) == 0 {
		return m.applyOutlineBuffer(pending)
	}
	// The counts are read here, from the project the buffer was dumped from, and
	// the apply refuses any revision that moved since. What the prompt names is
	// therefore still exactly what the commit removes.
	m.pendingOutlineApply = pending
	m.outlineDeleteConfirm.ShowWithDetail(
		outlineDeleteConfirmAction,
		outlineDeletePrompt(pending.project, pending.deletions),
		outlineDeleteConfirmDetail(pending.deletions),
	)
	return nil
}

// outlineApplyPending is one parsed buffer held while its deletions await
// confirmation. It carries the path so a declined apply can still name the file.
type outlineApplyPending struct {
	project      *models.Project
	revision     string
	path         string
	scopeTaskIDs []string
	lines        []app.OutlinePlanLine
	deletions    []*models.Task
}

const outlineDeleteConfirmAction = "outline-delete"

// applyOutlineBuffer submits one parsed buffer. Both routes into it — a buffer
// that deleted nothing and a confirmed deletion — run this same code, so the
// confirmation decides whether the apply happens and never what it does.
func (m *AppModel) applyOutlineBuffer(pending outlineApplyPending) tea.Cmd {
	// The dump revision travels with the request rather than being re-read here:
	// a check on this side of the call could only ever be a check-then-apply, and
	// the apply is where the tasks are actually mutated.
	result, err := m.features.Outline.Apply(app.OutlinePlanRequest{
		ProjectID:        pending.project.ID,
		ExpectedRevision: pending.revision,
		ScopeTaskIDs:     pending.scopeTaskIDs,
		DeleteTaskIDs:    taskIDsOf(pending.deletions),
		ConfirmDeletions: len(pending.deletions) > 0,
		Lines:            pending.lines,
	})
	if errors.Is(err, storage.ErrStaleRevision) {
		m.setWarningStatus("Workspace changed while the editor was open · buffer kept at " + pending.path)
		return defaultStatusTimeout()
	}
	if err != nil {
		m.showView(ViewOutlineErrors, NewOutlineErrorsViewModel(pending.project.Name, pending.path, []lineError{outlineApplyError(err)}))
		return nil
	}

	removeOutlineBuffer(pending.path)
	return tea.Batch(
		m.setRecoveryFeedback(result.Summary(), outlineTransactionID(result), false),
		func() tea.Msg { return SaveCompleteMsg{} },
	)
}

// handleOutlineDeleteConfirmKey gives the outline deletion the same modal key
// priority as the canonical task delete: while it is active it consumes every
// key, so nothing leaks into the Outliner underneath.
func (m *AppModel) handleOutlineDeleteConfirmKey(key string) (bool, tea.Cmd) {
	if !m.outlineDeleteConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.outlineDeleteConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		pending := m.pendingOutlineApply
		m.clearPendingOutlineApply()
		return true, m.applyOutlineBuffer(pending)
	}
	switch key {
	case "n", "N", "esc":
		path := m.pendingOutlineApply.path
		m.clearPendingOutlineApply()
		// Declining cancels the whole apply, edits included: a buffer that half
		// landed would leave the user guessing which half. The file stays, so the
		// same edits can be resubmitted after the removals are put back.
		m.setWarningStatus("Outline not applied · buffer kept at " + path)
		return true, defaultStatusTimeout()
	}
	return true, nil
}

func (m *AppModel) clearPendingOutlineApply() {
	m.pendingOutlineApply = outlineApplyPending{}
	m.outlineDeleteConfirm.Cancel()
}

// outlineDeletedTasks names the dumped tasks no line claims, in dump order.
func outlineDeletedTasks(project *models.Project, scopeTaskIDs []string, lines []app.OutlinePlanLine) []*models.Task {
	buffered := make(map[string]bool, len(lines))
	for _, line := range lines {
		if line.TaskID != "" {
			buffered[line.TaskID] = true
		}
	}
	deleted := make([]*models.Task, 0, len(scopeTaskIDs))
	for _, taskID := range scopeTaskIDs {
		if buffered[taskID] {
			continue
		}
		if task := project.GetTaskByID(taskID); task != nil {
			deleted = append(deleted, task)
		}
	}
	return deleted
}

func outlineDeletePrompt(project *models.Project, deletions []*models.Task) string {
	if len(deletions) == 1 {
		return fmt.Sprintf("Delete task #%d from %s?", deletions[0].ShortID, project.Name)
	}
	return fmt.Sprintf("Delete %d tasks from %s?", len(deletions), project.Name)
}

// outlineDeleteConfirmDetail describes the removal the buffer asked for. It
// carries no root/descendant split: every deletion here is a line the user
// removed themselves, so there is no cascade to warn about.
func outlineDeleteConfirmDetail(deletions []*models.Task) *ConfirmDetail {
	items := make([]string, 0, min(len(deletions), 5))
	for _, task := range deletions {
		if len(items) == 5 {
			break
		}
		items = append(items, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
	}
	stored, linked := countAttachmentImpact(deletions)
	noun := "tasks"
	if len(deletions) == 1 {
		noun = "task"
	}
	return &ConfirmDetail{
		Items:       items,
		TotalCount:  len(deletions),
		CascadeNote: fmt.Sprintf("%d %s deleted · %s", len(deletions), noun, attachmentImpactNote(stored, linked)),
		UndoHint:    true,
	}
}

func taskIDsOf(tasks []*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

// outlineTransactionID reads the committed transaction off any result the
// commit produced. Created, updated, and deleted results all carry the same
// one, because the whole buffer is a single transaction.
func outlineTransactionID(result app.OutlinePlanResult) string {
	for _, group := range [][]app.TaskResult{result.Created, result.Updated, result.Deleted} {
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
