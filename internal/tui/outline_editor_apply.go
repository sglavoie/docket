package tui

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleOpenOutlineEditor dumps the project and hands the terminal to $EDITOR.
func (m *AppModel) handleOpenOutlineEditor(msg OpenOutlineEditorMsg) tea.Cmd {
	if msg.Project == nil {
		return status.ErrorCmd("no project to outline")
	}
	modeLabel := ""
	if out := m.activeOutliner(); out != nil {
		if mode, ok := chrome.OutlinerModeByID(out.PrimaryMode); ok {
			modeLabel = mode.Label
		}
	}
	project, revision, buffer, scopeTaskIDs, err := m.outlineDumpSource(msg.Project.ID, modeLabel)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	path, err := external.CreateTempFileWithContent(buffer, outlineBufferPrefix, outlineBufferSuffix)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	editor, editorErr := external.ResolveEditor("")
	if editorErr != nil {
		removeOutlineBuffer(path)
		return status.ErrorCmd("%s", editorErr)
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
		return status.ErrorCmd("%s", msg.Err)
	}
	if msg.Project == nil {
		removeOutlineBuffer(msg.Path)
		return status.ErrorCmd("no project to outline")
	}

	lines, lineErrors := grammar.ParseOutlineBuffer(msg.Buffer, msg.Project, m.outlineDependencyProjects(msg.Project))
	if len(lineErrors) > 0 {
		m.showView(presentation.ViewOutlineErrors, outliner.NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, lineErrors))
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
		m.showView(presentation.ViewOutlineErrors, outliner.NewOutlineErrorsViewModel(msg.Project.Name, msg.Path, []grammar.LineError{{Line: 1, Message: "the buffer holds no tasks"}}))
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
		return status.DefaultStatusTimeout()
	}
	if err != nil {
		m.showView(presentation.ViewOutlineErrors, outliner.NewOutlineErrorsViewModel(pending.project.Name, pending.path, []grammar.LineError{outlineApplyError(err)}))
		return nil
	}

	removeOutlineBuffer(pending.path)
	return tea.Batch(
		m.setRecoveryFeedback(result.Summary(), outlineTransactionID(result), false),
		func() tea.Msg { return status.SaveCompleteMsg{} },
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
		return true, status.DefaultStatusTimeout()
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
func outlineDeleteConfirmDetail(deletions []*models.Task) *chrome.ConfirmDetail {
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
	return &chrome.ConfirmDetail{
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
func outlineApplyError(err error) grammar.LineError {
	message := err.Error()
	tail, prefixed := strings.CutPrefix(message, "line ")
	if !prefixed {
		return grammar.LineError{Line: 1, Message: message}
	}
	number, detail, split := strings.Cut(tail, ": ")
	parsed, convErr := strconv.Atoi(number)
	if !split || convErr != nil || parsed <= 0 {
		return grammar.LineError{Line: 1, Message: message}
	}
	return grammar.LineError{Line: parsed, Message: detail}
}

// removeOutlineBuffer discards a temp file the user no longer needs. A failed
// removal is not worth a status: the file is in the system temp directory and
// the edit itself already succeeded or failed on its own terms.
func removeOutlineBuffer(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
