package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate to specialized handlers by category

	// CRUD operations
	if handled, cmd := m.handleTaskCRUDMessages(msg); handled {
		return handled, cmd
	}

	// Parent selector messages
	if handled, cmd := m.handleTaskParentMessages(msg); handled {
		return handled, cmd
	}

	// View-opening messages (detail, comments)
	if handled, cmd := m.handleTaskViewMessages(msg); handled {
		return handled, cmd
	}

	// Comment messages
	if handled, cmd := m.handleTaskCommentMessages(msg); handled {
		return handled, cmd
	}

	// Command palette task actions
	// Content editing messages
	if handled, cmd := m.handleTaskContentMessages(msg); handled {
		return handled, cmd
	}

	return false, nil
}

// handleTaskContentMessages handles task content editing messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskContentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case EditTaskContentMsg:
		return m.handleEditTaskContent(msg)

	case TaskContentEditedMsg:
		return m.handleTaskContentEdited(msg)

	case OpenOutlineEditorMsg:
		return true, m.handleOpenOutlineEditor(msg)

	case OutlineEditedMsg:
		return true, m.handleOutlineEdited(msg)

	case coordinator.TaskLineEditedMsg:
		return true, m.handleTaskLineEdited(msg)

	case coordinator.TaskLineEditFailedMsg:
		return true, m.handleTaskLineEditFailed(msg)

	}
	return false, nil
}

// handleTaskViewMessages handles task-related view opening messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskdetail.ShowTaskCommentsMsg:
		return m.handleShowTaskComments(msg)

	}
	return false, nil
}

// handleShowTaskComments handles the ShowTaskCommentsMsg.
func (m *AppModel) handleShowTaskComments(msg taskdetail.ShowTaskCommentsMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}
	owner := taskdetail.NewTaskCommentOwner(msg.Task, msg.Project, m.secondaryFeatures.Subview)
	view := taskdetail.NewCommentsPanel(owner, "")
	view.SetOperationDispatch(m.dispatchOperationKey)
	return m.showViewInit(presentation.ViewTaskComments, view)
}

// handleTaskParentMessages handles task parent selector messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskParentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.OpenParentSelectorMsg:
		return m.handleOpenParentSelector(msg)

	case listsurface.ParentSelectedMsg:
		return m.handleParentSelected(msg)
	}
	return false, nil
}

// handleOpenParentSelector handles the OpenParentSelectorMsg.
func (m *AppModel) handleOpenParentSelector(msg taskform.OpenParentSelectorMsg) (bool, tea.Cmd) {
	selector := listsurface.NewParentSelectorModel(msg.ValidParents, msg.CurrentID, msg.AllowNone, msg.Source)
	selector.ReparentTask = msg.ReparentTask
	_, cmd := m.showViewInit(presentation.ViewParentSelector, selector)
	return true, cmd
}

// handleParentSelected handles the ParentSelectedMsg.
func (m *AppModel) handleParentSelected(msg listsurface.ParentSelectedMsg) (bool, tea.Cmd) {
	// Pop the parent selector view
	m.popView()
	// Forward the message directly to the underlying view (note/task creator)
	return true, m.delegateToCurrentView(msg)
}

// handleTaskCommentMessages handles task comment messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskCommentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskdetail.CommentAddedMsg:
		return m.handleCommentAdded(msg)

	case taskdetail.CommentDeletedMsg:
		return m.handleCommentDeleted(msg)

	case taskdetail.CommentEditedMsg:
		return m.handleCommentEdited(msg)

	case taskdetail.CommentMutationFailedMsg:
		return true, m.handleCommentMutationFailed(msg)

	}
	return false, nil
}

// refreshAfterSubviewCommit re-syncs the task views after a subview panel
// mutation committed through the application layer. The commit loaded a fresh
// snapshot into the shared project pointers, so views caching task pointers
// (tree nodes, outliner rows) must re-resolve to observe the committed objects
// — the raw-save path they replaced mutated the shared objects in place and
// never needed this.
func (m *AppModel) refreshAfterSubviewCommit() tea.Cmd {
	return tea.Batch(m.refreshTaskViewsAfterSave()...)
}

// handleCommentAdded handles the CommentAddedMsg.
func (m *AppModel) handleCommentAdded(msg taskdetail.CommentAddedMsg) (bool, tea.Cmd) {
	// Delegate to current view so it can refresh stale task data
	// (outliner rows contain clones from tree projections)
	viewCmd := m.committedCommentViewCmd(msg.ProjectID, msg)
	return true, tea.Batch(m.setRecoveryFeedback("Comment added", msg.TransactionID, false), viewCmd, m.refreshAfterSubviewCommit())
}

// handleCommentDeleted handles the CommentDeletedMsg.
func (m *AppModel) handleCommentDeleted(msg taskdetail.CommentDeletedMsg) (bool, tea.Cmd) {
	viewCmd := m.committedCommentViewCmd(msg.ProjectID, msg)
	return true, tea.Batch(m.setRecoveryFeedback("Comment deleted", msg.TransactionID, false), viewCmd, m.refreshAfterSubviewCommit())
}

// handleCommentEdited handles the CommentEditedMsg.
func (m *AppModel) handleCommentEdited(msg taskdetail.CommentEditedMsg) (bool, tea.Cmd) {
	viewCmd := m.committedCommentViewCmd(msg.ProjectID, msg)
	return true, tea.Batch(m.setRecoveryFeedback("Comment updated", msg.TransactionID, false), viewCmd, m.refreshAfterSubviewCommit())
}

// committedCommentViewCmd reloads the committed project before the panel reads it
// back. The three comment outcomes replace the TransactionResultMsg the
// coordinator would otherwise have raised, so they owe the shell the same
// in-memory reload handleTransactionResult performs: the panel projects its
// owner's stored comments rather than editing them, and the application layer
// committed against its own freshly loaded snapshot, so without this the panel
// re-reads the pre-commit copy the shell still holds.
func (m *AppModel) committedCommentViewCmd(projectID string, msg tea.Msg) tea.Cmd {
	m.reloadProjectByID(projectID)
	return m.delegateToCurrentView(msg)
}

// handleCommentMutationFailed routes a refused comment commit back to the panel
// that submitted it. It mirrors the shared ErrorMsg handling — the same status
// surface, the same timeout — and adds the delegation the panel needs to put the
// user's draft back in front of them; a panel that has since moved on drops it.
func (m *AppModel) handleCommentMutationFailed(msg taskdetail.CommentMutationFailedMsg) tea.Cmd {
	m.feedback.err = msg.Err
	return tea.Batch(m.delegateToCurrentView(msg), status.StatusTimeout(5*time.Second))
}

func (m *AppModel) handleTaskLineEditFailed(msg coordinator.TaskLineEditFailedMsg) tea.Cmd {
	m.feedback.err = msg.Err
	message := "task edit failed"
	if msg.Err != nil {
		message = msg.Err.Error()
	}
	if out, ok := m.outlinerPrimary(); ok && out != nil && out.QuickEdit.IsActive() {
		task := out.QuickEdit.GetTask()
		project := out.QuickEdit.GetProject()
		if task != nil && project != nil && taskref.For(task, project) == msg.Source {
			out.QuickEdit.RejectCommit(message)
		}
	}
	return status.StatusTimeout(5 * time.Second)
}

func (m *AppModel) handleTaskLineEdited(msg coordinator.TaskLineEditedMsg) tea.Cmd {
	m.feedback.err = nil
	out, ok := m.outlinerPrimary()
	if ok && out != nil && out.QuickEdit.IsActive() {
		task := out.QuickEdit.GetTask()
		project := out.QuickEdit.GetProject()
		if task != nil && project != nil && taskref.For(task, project) == msg.Source {
			out.QuickEdit.Exit()
		}
	}

	if !msg.Result.Changed {
		m.setInfoStatus("No task changes")
		return status.DefaultStatusTimeout()
	}
	for _, projectID := range msg.Result.AffectedProjectIDs {
		m.reloadProjectByID(projectID)
		m.reloadCrossProjectOutlinerProject(projectID)
	}
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()

	recordedOrigin := false
	var jumpCmd tea.Cmd
	if ok && out != nil {
		final := taskref.TaskIdentity{
			ProjectID: msg.Result.Task.ProjectID,
			TaskID:    msg.Result.Task.TaskID,
		}
		jumpCmd, recordedOrigin = out.FinishTaskLineEditRelocation(outliner.OutlinerRelocationFrom(msg.Relocation), final)
	}

	summary := fmt.Sprintf("Updated #%d %s", msg.Result.ShortID, msg.Result.Title)
	if msg.Result.MovedDescendantCount > 0 {
		summary = fmt.Sprintf("Updated #%d and moved %d descendant(s)", msg.Result.ShortID, msg.Result.MovedDescendantCount)
	}
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(summary, msg.Result.TransactionID, recordedOrigin)
	return tea.Batch(saveCmd, jumpCmd, feedbackCmd)
}
