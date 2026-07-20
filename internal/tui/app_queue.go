package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// QueueToggleable is implemented by views that can provide a task for queue toggling.
type QueueToggleable interface {
	getTaskRef() (*models.Task, *models.Project)
}

// taskIsNext reports Next (queue) membership. Membership is the StateNext
// workflow state, not the persisted Focus Queue order index — that list is
// consulted only to order StateNext tasks. An unindexed StateNext task is a
// member; a stale indexed non-Next task is not.
func taskIsNext(task *models.Task) bool {
	return task != nil && task.State() == models.StateNext
}

// queueToggleAction records a single q-key toggle for undo support. Queue
// membership is now the StateNext workflow state, so the toggle also records the
// task's pre-toggle state (PrevState) so the compensating undo can restore both
// the workflow state and the Next order-list entry atomically.
type queueToggleAction struct {
	Task           *models.Task
	Project        *models.Project
	WasAdd         bool                 // true = task was added to queue
	WasInDailyPlan bool                 // true = task was in daily plan before (restore on undo)
	PrevState      models.WorkflowState // task's workflow state before the toggle
}

// removeFromDailyPlan drops a task from the in-memory daily plan.
func (m *AppModel) removeFromDailyPlan(taskID string) {
	for i, ft := range m.dailyPlan {
		if ft.Task.ID == taskID {
			m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
			break
		}
	}
	delete(m.dailyPlanIDs, taskID)
}

// queueToggleResultCmd fans the state change out to every task view (mirroring a
// committed save) and schedules the status-line timeout, without clearing the
// toggle-undo history the toggle just recorded.
func (m *AppModel) queueToggleResultCmd() tea.Cmd {
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, defaultStatusTimeout())
	return tea.Batch(cmds...)
}

// toggleQueue adds or removes the focused task from the Next queue. Adding sets
// the task's workflow state to StateNext, drops any Today (Daily Plan) entry, and
// appends the Next order entry — all folded into ONE workspace-ledger transaction
// via focusQueueAddArtifactBuilder. Removing transitions the task out of StateNext
// (to StateReady), which drops the stale order entry through the service's
// exclusivity reconciliation in the same transaction. Because the whole move is a
// single durable ledger entry, undo/redo route through the ledger
// (undoQueueToggle/redoQueueToggle) rather than a bespoke in-memory compensation,
// so they survive a restart and honor LIFO ordering. Returns a status timeout cmd
// on success, or nil if no task is available.
func (m *AppModel) toggleQueue() tea.Cmd {
	view := m.views.Get(m.currentView())
	if view == nil {
		return nil
	}
	qt, ok := view.(QueueToggleable)
	if !ok {
		return nil
	}
	task, project := qt.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	taskID, projectID := task.ID, project.ID

	if taskIsNext(task) {
		// Remove from queue: leave StateNext for StateReady. The service's
		// exclusivity reconciliation drops the stale order entry in the same
		// ledger transaction.
		fresh, _, err := m.queueLedgerTransition(taskID, projectID, models.StateReady, false)
		if err != nil {
			return errorCmd("%s", err)
		}
		title := taskTitleOrFallback(fresh, task)
		m.setInfoStatus(fmt.Sprintf("Removed from queue: %s", title))
	} else {
		// Add to queue: enter StateNext, drop Today, and append the order entry as
		// one ledger transaction.
		fresh, _, err := m.queueLedgerTransition(taskID, projectID, models.StateNext, true)
		if err != nil {
			return errorCmd("%s", err)
		}
		title := taskTitleOrFallback(fresh, task)
		m.setInfoStatus(fmt.Sprintf("Added to queue: %s", title))
	}
	m.lastToggleUndoKind = toggleUndoQueue
	m.lastQueueToggleAt = time.Now()
	m.clearToggleRedoHistory()
	return m.queueToggleResultCmd()
}

// queueLedgerTransition commits a Next-queue promotion or removal as one ledger
// transaction that folds the workflow-state change and the planning change
// (Today/Next exclusivity plus, when appendOrder is set, the Next order entry)
// together, then reloads the touched project and config and rebuilds the
// in-memory planning collections from the committed state.
func (m *AppModel) queueLedgerTransition(taskID, projectID string, target models.WorkflowState, appendOrder bool) (*models.Task, *models.Project, error) {
	var builder app.PlanningArtifactBuilder
	if appendOrder {
		builder = focusQueueAddArtifactBuilder(m.configManager, taskID, projectID)
	} else {
		builder = planningReconcileArtifactBuilder(m.configManager)
	}
	res, err := m.appService.Transition(app.StateRequest{TaskID: taskID, Target: target, PlanningArtifacts: builder})
	if err != nil {
		return nil, m.projectByID(projectID), err
	}
	m.reloadProjectByID(res.ProjectID)
	m.refreshRuntime()
	m.rebuildPlanningFromRuntime()
	project := m.projectByID(res.ProjectID)
	var fresh *models.Task
	if project != nil {
		fresh = project.GetTaskByID(taskID)
	}
	return fresh, project, nil
}

// undoQueueToggle reverses the last Next-queue toggle by undoing its durable
// workspace-ledger transaction, restoring the workflow state, Today membership,
// and Next ordering together. Unlike the retired in-memory compensation, it reads
// the durable ledger, so it works after a restart and always undoes the newest
// transaction first.
func (m *AppModel) undoQueueToggle() tea.Cmd {
	m.lastQueueToggle = nil
	m.lastQueueToggleAt = time.Time{}
	// The workspace ledger is the durable LIFO history: keep routing u/ctrl+r
	// through it so consecutive presses walk the stack (newest first) and a
	// following ctrl+r replays, mirroring the persistent undo contract.
	m.lastToggleRedoKind = toggleUndoQueue
	m.lastQueueRedoAt = time.Now()
	m.lastToggleUndoKind = toggleUndoQueue
	return m.planningLedgerRestore(false)
}

// redoQueueToggle replays the last undone Next-queue toggle through the workspace
// ledger's redo stack.
func (m *AppModel) redoQueueToggle() tea.Cmd {
	m.lastQueueToggleRedo = nil
	m.lastQueueRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoQueue
	m.lastToggleUndoKind = toggleUndoQueue
	m.lastQueueToggleAt = time.Time{}
	return m.planningLedgerRestore(true)
}

// taskTitleOrFallback returns the fresh task title, falling back to the
// pre-reload pointer when the reload could not resolve the task.
func taskTitleOrFallback(fresh, fallback *models.Task) string {
	if fresh != nil {
		return fresh.Title
	}
	if fallback != nil {
		return fallback.Title
	}
	return ""
}

// persistFocusQueue saves the current Next order index through a checked planning
// commit. The order index is planning-domain state, so it is committed to
// planning.json alone rather than routed through a whole-config save.
func (m *AppModel) persistFocusQueue() {
	items := make([]config.FocusQueueItem, len(m.focusQueue))
	for i, ft := range m.focusQueue {
		items[i] = config.FocusQueueItem{
			TaskID:    ft.Task.ID,
			ProjectID: ft.Project.ID,
		}
	}
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.SetFocusQueue(items)
	}); err != nil {
		m.err = err
	}
}
