package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// handleTransactionResult routes a committed feature-coordinator mutation back
// into the shell. The application layer committed one atomic ChangeSet against
// its own freshly loaded snapshot, so the AppModel's in-memory copy of the
// affected project is reloaded from disk before the shared post-save refresh
// runs, keeping every open view consistent with the committed transaction.
func (m *AppModel) handleTransactionResult(msg coordinator.TransactionResultMsg) tea.Cmd {
	m.reloadProjectByID(msg.Result.ProjectID)
	if m.crossOutliner != nil {
		m.crossOutliner.ArmOutlinerRelocation(outliner.OutlinerRelocationFrom(msg.Relocation))
	}
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.SettleRelocationCursor()
	}
	m.observeCommittedDueEffect(msg.Result.TransactionID, msg.DueEffect)
	// A single-task result already names its one target, at the address the commit
	// left it at; the move map is what recovers where it started.
	m.undoFocus.ObserveCommit(msg.Result.TransactionID, coordinator.FocusTargetFromResult(
		app.TaskLocator{ProjectID: msg.Result.ProjectID, TaskID: msg.Result.TaskID}, msg.DueEffect.Moves))
	m.revealCommittedChecklistItemAt(msg.RevealChecklistFor, msg.RevealChecklistIndex)
	m.revealCommittedTask(msg.RevealTaskFor, taskref.TaskIdentity{
		ProjectID: msg.Result.ProjectID,
		TaskID:    msg.Result.TaskID,
	})
	saveCmd := m.handleCommittedSaveComplete()
	if panel := m.entities.ActiveAttachmentsPanel(m); panel != nil {
		panel.FollowAttachmentReorder(taskref.TaskIdentity{
			ProjectID: msg.Result.ProjectID, TaskID: msg.Result.TaskID,
		}, msg.AttachmentReorder)
	}
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage(), msg.Result.TransactionID, false)
	return tea.Batch(saveCmd, feedbackCmd)
}

// revealCommittedChecklistItemAt opens the owner's checklist and seats the
// cursor on the item the transaction appended. It runs from the success message
// alone and after the workspace refresh, so the fold and the cursor move only
// once the committed item actually exists in a rebuilt projection; a refused
// append never reaches here and leaves the outline exactly as it was. A nil
// inserted index means the append landed at the end.
func (m *AppModel) revealCommittedChecklistItemAt(identity taskref.TaskIdentity, inserted *int) {
	if m == nil || m.crossOutliner == nil || !identity.Valid() {
		return
	}
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil || len(task.Checklist) == 0 {
		return
	}
	m.crossOutliner.Folds.SetChecklist(identity, true)
	m.crossOutliner.RebuildRows()
	index := len(task.Checklist) - 1
	if inserted != nil {
		index = *inserted
	}
	if !m.crossOutliner.MoveCursorToChecklistItem(identity, index) && inserted != nil {
		m.crossOutliner.MoveCursorToIdentity(identity)
		m.crossOutliner.SetTransientStatus("Checklist item was created but is hidden by this view")
	}
}

// revealCommittedTask seats the cursor on a task this transaction minted, named
// by qualified identity because the row the gesture was made on no longer
// exists: checklist promotion consumes the item it converts, so the child is the
// only thing left to point at and only the commit knows what it is.
//
// It runs from the success message alone and after the reload, so the cursor
// moves only once the child is really in a rebuilt projection. It opens the
// branch that owns the child and nothing else — scope, mode, query, sections and
// every unrelated fold are left as the user had them — which means the current
// view may legitimately refuse to show the child. That is reported rather than
// worked around: the cursor falls back to the owner, or to whatever the clamp
// leaves under it, and the status names the child the user cannot see.
func (m *AppModel) revealCommittedTask(identity, owner taskref.TaskIdentity) {
	if m == nil || m.crossOutliner == nil || !identity.Valid() {
		return
	}
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return
	}
	out := m.crossOutliner
	if out.ExpandAncestorsOfTask(task, identity.ProjectID) {
		out.RebuildRows()
	}
	if out.MoveCursorToIdentity(identity) {
		return
	}
	if owner.Valid() && owner != identity {
		out.MoveCursorToIdentity(owner)
	}
	out.SetTransientStatus(hiddenCommittedTaskStatus(task))
}

// hiddenCommittedTaskStatus names a committed task the current view does not
// admit. A promoted item that was already ticked becomes a done subtask, and
// done work is hidden by default, so that case says so outright instead of
// leaving the user to wonder whether anything was written at all.
func hiddenCommittedTaskStatus(task *models.Task) string {
	title := strings.TrimSpace(task.Title)
	if task.IsTerminalState() {
		return fmt.Sprintf("Subtask %q was created %s and is hidden by this view", title, task.State())
	}
	return fmt.Sprintf("Subtask %q was created and is hidden by this view", title)
}

// handleBatchTransactionResult routes a committed project-scoped or multi-task
// coordinator mutation (bulk transition/delete or metadata update) back into the
// shell. The application layer committed one atomic ChangeSet across every touched
// project, so each affected project's in-memory copy is reloaded from disk before
// the shared post-save refresh runs, keeping every open view consistent.
func (m *AppModel) handleBatchTransactionResult(msg coordinator.BatchTransactionResultMsg) tea.Cmd {
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}
	if m.crossOutliner != nil {
		m.crossOutliner.ArmOutlinerRelocation(outliner.OutlinerRelocationFrom(msg.Relocation))
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.SettleRelocationCursor()
	}
	m.observeCommittedDueEffect(msg.TransactionID, msg.DueEffect)
	// Only a batch that carries one named target correlates. A bulk gesture leaves
	// the zero value here, which records nothing, which is what keeps a later undo
	// of it from moving the cursor out of the selection the user still holds.
	m.undoFocus.ObserveCommit(msg.TransactionID,
		coordinator.FocusTargetFromSource(msg.CommittedTarget, msg.DueEffect.Moves))
	// Clear multiselect state only now that the batch has committed, so a rejected
	// batch (TransactionErrorMsg, which never reaches this handler) leaves the
	// selection intact for the user to retry.
	if len(msg.ClearSelectionTaskLocators) > 0 {
		m.clearBulkTaskSelectionsForLocators(msg.ClearSelectionTaskLocators)
	} else if len(msg.ClearSelectionTaskIDs) > 0 {
		m.clearBulkTaskSelectionsForTaskIDs(msg.ClearSelectionTaskIDs)
	}
	// The checked planning artifact folded into this batch is now committed.
	// Rebuild the shared planning caches only after the runtime snapshot advances:
	// registered views refresh below and must never render a known-stale snapshot.
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	successMessage := msg.SuccessMessage()
	if successMessage == "" {
		successMessage = m.feedback.message
	}
	saveCmd := m.handleCommittedSaveComplete()
	feedbackCmd := m.setRecoveryFeedbackWithContext(successMessage, msg.TransactionID,
		recoveryWords{Context: msg.RecoveryContext, Headline: msg.RecoveryHeadline}, false)
	return tea.Batch(saveCmd, feedbackCmd)
}

func (m *AppModel) refreshLoadedTaskWorkspace() {
	if m.crossOutliner != nil {
		outliner.RebuildOutlinerPreservingCursor(m.crossOutliner)
	}
}

// reloadProjectByID reloads a project's tasks from disk into the AppModel's
// in-memory copy so views bound to it reflect a mutation committed through the
// application layer. A blank id, unknown project, or load error is a no-op — the
// shared post-save refresh reloads the current view's projects regardless.
func (m *AppModel) reloadProjectByID(projectID string) bool {
	if projectID == "" {
		return false
	}
	project := m.projectByID(projectID)
	if project == nil {
		return false
	}
	if err := m.reloadCommittedProject(project); err != nil {
		logging.Warn("failed to reload project %s after transaction: %v", project.Name, err)
		return false
	}
	return true
}

// reloadCommittedProject reloads a shell-cached project after a commit through
// the Outliner's loader and copies the result into the cached object, the way
// adoptCrossProjectSnapshot adopts a load. The loader keeps the copy it read as
// its cached parse, so the reload the commit's refresh starts is answered with
// the very task slice the handler's rebuild renders, and publishing it tells the
// Outliner nothing has changed. The copy, not the cached object, goes to the
// loader because the reload reads what it caches on a command goroutine.
func (m *AppModel) reloadCommittedProject(project *models.Project) error {
	if m.crossOutliner == nil || m.crossOutliner.ProjectService == nil {
		return storage.LoadProject(project)
	}
	fresh := *project
	if err := m.crossOutliner.ProjectService.WorkspaceLoader().Reload(&fresh); err != nil {
		return err
	}
	*project = fresh
	return nil
}

func (m *AppModel) handleGlobalRefreshMessage(tea.Msg) (bool, tea.Cmd) { return false, nil }

func (m *AppModel) handleSaveComplete(msg status.SaveCompleteMsg) tea.Cmd {
	return m.completeSave(msg, false)
}

// handleCommittedSaveComplete is handleSaveComplete for a transaction handler
// that has already reloaded the projects its commit touched and rebuilt the
// Outliner over them. The shared refresh then asks the Outliner's memo guard
// instead of forcing a second rebuild of the same rows, so it rebuilds only when
// something the guard reads moved after the handler's rebuild — the planning a
// batch refreshes afterwards, for one.
func (m *AppModel) handleCommittedSaveComplete() tea.Cmd {
	return m.completeSave(status.SaveCompleteMsg{}, true)
}

func (m *AppModel) completeSave(msg status.SaveCompleteMsg, outlinerRebuilt bool) tea.Cmd {
	m.clearToggleHistory()
	// An empty success message means the mutation carried no status of its own
	// (e.g. a ProjectTree/Outliner reorder committed through CommitProjectWorkflow):
	// leave any status the view set synchronously intact rather than blanking it.
	if msg.SuccessMessage != "" {
		m.setSuccessStatus(msg.SuccessMessage)
	}
	saveCmds := m.refreshTaskViewsOver(true, true, outlinerRebuilt)
	saveCmds = append(saveCmds, m.statusReadingTimeout())
	return tea.Batch(saveCmds...)
}

func (m *AppModel) refreshTaskViewsAfterSave() []tea.Cmd {
	return m.refreshTaskViews(true, true)
}

// refreshTaskViewsAfterPlanningMutation fans planning changes out through the
// already-loaded task views. The application service has synchronously reloaded
// every touched project and planning snapshot, so starting an asynchronous
// cross-project reload here would blank live rows until its command is run.
func (m *AppModel) refreshTaskViewsAfterPlanningMutation() []tea.Cmd {
	return m.refreshTaskViews(true, false)
}

// refreshTaskViewsAfterRestore uses the same cache/view fan-out as a committed
// save. When the runtime reload failed, it rebuilds the current project tree in
// memory without persisting view state from the stale runtime snapshot.
func (m *AppModel) refreshTaskViewsAfterRestore(runtimeReloaded bool) []tea.Cmd {
	return m.refreshTaskViews(runtimeReloaded, true)
}

func (m *AppModel) refreshTaskViews(persistProjectTreeState, reloadCrossProject bool) []tea.Cmd {
	return m.refreshTaskViewsOver(persistProjectTreeState, reloadCrossProject, false)
}

// refreshTaskViewsOver is refreshTaskViews for a caller that may already have
// rebuilt the Outliner over the reloaded projects, in which case the memo guard
// decides whether anything is left to rebuild.
func (m *AppModel) refreshTaskViewsOver(persistProjectTreeState, reloadCrossProject, outlinerRebuilt bool) []tea.Cmd {
	m.refreshWorkspacePulseAt(time.Now())
	if outlinerRebuilt {
		outliner.RefreshOutlinerPreservingCursor(m.crossOutliner)
	} else {
		m.refreshProjectViewAfterTaskMutation(persistProjectTreeState)
	}
	var saveCmds []tea.Cmd
	if reloadCrossProject {
		if reloadCmd := m.requestCrossProjectOutlinerReload(); reloadCmd != nil {
			saveCmds = append(saveCmds, reloadCmd)
		}
	}
	if m.viewNav.views != nil {
		for _, viewType := range []presentation.PresentationID{
			presentation.ViewTaskComments,
			presentation.ViewTaskAttachments,
		} {
			view := m.viewNav.views.Get(viewType)
			refresher, ok := view.(interface{ RefreshAfterReload() bool })
			if !ok {
				continue
			}
			if !refresher.RefreshAfterReload() && m.currentView() == viewType {
				saveCmds = append(saveCmds, func() tea.Msg { return status.BackMsg{} })
			}
		}
	}
	return saveCmds
}

func (m *AppModel) refreshWorkspacePulseAt(now time.Time) {
	cachedActivityIsCurrent := m.completionActivity.IsCurrentLocalDay(now)
	m.workspacePulse, m.workspaceAgenda, m.workspaceUpcoming = outliner.ComputeWorkspaceReadout(m.domain.runtime, m.todayIDs, now, m.ensurePulseLoader(), m.control.enabled())
	m.refreshControlFocus(now)
	activity, err := m.features.Activity.CompletionActivity()
	if err != nil {
		logging.Warn("refresh completion activity: %v", err)
		if cachedActivityIsCurrent {
			m.workspacePulse.Done = m.completionActivity.CompletedToday()
		} else {
			m.completionActivity = app.CompletionActivity{}
			m.workspacePulse.Done = 0
		}
		return
	}
	m.completionActivity = activity
	m.workspacePulse.Done = activity.CompletedToday()
}

// refreshWorkspacePulseAcrossLocalDay refreshes a pulse whose cached completion
// activity belongs to an earlier local day. A wake still owed here was refused
// for interaction safety, so its task remains hidden from the outline and only
// Done may move: holding Inbox, Today, and Overdue keeps the counts and the rows
// they describe in step until the rebuild is allowed.
func (m *AppModel) refreshWorkspacePulseAcrossLocalDay(now time.Time) {
	held := m.workspacePulse
	m.refreshWorkspacePulseAt(now)
	if !m.deferWakeOwedAt(now) {
		return
	}
	m.workspacePulse.Inbox = held.Inbox
	m.workspacePulse.Today = held.Today
	m.workspacePulse.Overdue = held.Overdue
}

func (m *AppModel) refreshProjectViewAfterTaskMutation(persistProjectTreeState bool) {
	if persistProjectTreeState {
		m.refreshProjectView()
	} else {
		m.refreshProjectViewInMemory()
	}
}
