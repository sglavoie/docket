package tui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
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

	case taskdetail.ShowProjectCommentsMsg:
		return m.handleShowProjectComments(msg)

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

// handleShowProjectComments mounts the same Comments panel over a project.
//
// The owner is handed the shell's project resolver rather than the project the
// message carried, because a committed transaction replaces the in-memory
// project object: the panel must survive its own writes, an undo, a rename, and
// a tag change, and it can only do that by re-reading the identity it was opened
// on. The resolver reads the shell's stable project cache, which is every
// project the catalog holds, so a parked project stays reachable here exactly as
// it is everywhere else.
func (m *AppModel) handleShowProjectComments(msg taskdetail.ShowProjectCommentsMsg) (bool, tea.Cmd) {
	if msg.Project == nil {
		return true, nil
	}
	owner := taskdetail.NewProjectCommentOwner(msg.Project, m.projectByID, m.secondaryFeatures.ProjectComment)
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
	return m.commentOutcomeCmd(msg)
}

// commentOutcomeCmd hands one comment outcome to the panel that submitted it
// rather than to whatever presentation happens to be on top.
//
// The submitting panel is not always the current view when its own commit
// answers. `?` raises Help, and Help is an overlay, so the panel underneath is
// still mounted, still holding the pending submission, and still the only model
// that can settle it. Delegating to the current view would hand the outcome to
// Help, which has no use for it; the panel would then keep refusing every later
// add, edit, and delete with the duplicate-submission refusal, because nothing
// else clears pending — RefreshAfterReload deliberately does not. Owner and token
// still decide whether the panel keeps what arrives, so a dismissed panel and a
// replacement owner drop a stale outcome exactly as they did before.
//
// The fallback is not vestigial: `C` on an Outliner row commits an add with no
// panel mounted at all, and the Outliner consumes that message to rebuild its
// rows.
func (m *AppModel) commentOutcomeCmd(msg tea.Msg) tea.Cmd {
	if m.commentsPanelMounted() {
		return m.viewNav.views.Update(presentation.ViewTaskComments, msg)
	}
	return m.delegateToCurrentView(msg)
}

// commentsPanelMounted reports whether the Comments panel is still on the route,
// covered or not. The registry alone cannot answer it: popping a view leaves its
// model registered, so a panel the user walked away from would go on collecting
// outcomes the current view is owed.
func (m *AppModel) commentsPanelMounted() bool {
	if m.viewNav.views == nil || !m.viewNav.views.Has(presentation.ViewTaskComments) {
		return false
	}
	return slices.Contains(m.presentationPath(), presentation.ViewTaskComments)
}

// handleCommentMutationFailed routes a refused comment commit back to the panel
// that submitted it. It mirrors the shared ErrorMsg handling — the same status
// surface, the same timeout — and adds the delegation the panel needs to put the
// user's draft back in front of them; a panel that has since moved on drops it.
func (m *AppModel) handleCommentMutationFailed(msg taskdetail.CommentMutationFailedMsg) tea.Cmd {
	m.feedback.err = msg.Err
	return tea.Batch(m.commentOutcomeCmd(msg), status.StatusTimeout(5*time.Second))
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

	// The typed effect is folded in ahead of the no-op return, because an explicit
	// due token is an override even when the deadline it names is the one the task
	// already had: the edit committed nothing, and the clock it overrode is still
	// no longer the user's intent.
	lineEditEffect := taskLineEditDueEffect(msg.Result)
	m.supersedeDueStepsForEffect(lineEditEffect)
	m.dueClocks.ObserveCommit(msg.Result.TransactionID, lineEditEffect)
	if !msg.Result.Changed {
		m.setInfoStatus("No task changes")
		return status.DefaultStatusTimeout()
	}
	// A row edit names both halves outright: the identity the editor was opened on
	// and the address the commit left the task at, which a `%project` token can
	// have moved to another project entirely. Unlike the clock memory above it is
	// recorded past the no-op return, because an edit that changed nothing wrote no
	// ledger entry for an undo to correlate with.
	m.undoFocus.ObserveCommit(msg.Result.TransactionID, coordinator.CommittedFocusTarget{
		Before: msg.Source.Locator(),
		After:  app.TaskLocator{ProjectID: msg.Result.Task.ProjectID, TaskID: msg.Result.Task.TaskID},
	})
	for _, projectID := range msg.Result.AffectedProjectIDs {
		m.reloadProjectByID(projectID)
		m.reloadCrossProjectOutlinerProject(projectID)
	}
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	m.reconcileRememberedDueClocks()

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
