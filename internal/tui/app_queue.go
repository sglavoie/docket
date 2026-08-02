package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// QueueToggleable is implemented by views that can provide a task for queue toggling.
type QueueToggleable interface {
	getTaskRef() (*models.Task, *models.Project)
}

// taskIsNext reports explicit project-qualified Next membership.
func (m *AppModel) taskIsNext(task *models.Task, project *models.Project) bool {
	if task == nil || project == nil {
		return false
	}
	return m.nextOrderIDs[taskIdentityFor(task, project)]
}

// queueToggleAction records a single q-key toggle for undo support. Next is an
// explicit planning membership; PrevState remains for the compatibility undo
// payload while the workspace ledger owns durable restoration.
type queueToggleAction struct {
	Task       *models.Task
	Project    *models.Project
	WasAdd     bool                 // true = task was added to queue
	WasInToday bool                 // true = task was in daily plan before (restore on undo)
	PrevState  models.WorkflowState // task's workflow state before the toggle
}

// removeFromToday drops exactly one project-qualified task from the
// in-memory daily plan.
func (m *AppModel) removeFromToday(identity TaskIdentity) {
	for i, ft := range m.today {
		if taskIdentityFor(ft.Task, ft.Project) == identity {
			m.today = append(m.today[:i], m.today[i+1:]...)
			break
		}
	}
	delete(m.todayIDs, identity)
}

// queueToggleResultCmd fans the state change out to every task view (mirroring a
// committed save) and schedules the status-line timeout, without clearing the
// toggle-undo history the toggle just recorded.
func (m *AppModel) queueToggleResultCmd() tea.Cmd {
	// Planning actions commit synchronously before returning here. Rebuild the
	// loaded Outliner immediately so Inbox/Next/Backlog rows disappear and the
	// cursor advances before the next render, without waiting for an async reload.
	m.refreshLoadedTaskWorkspace()
	cmds := m.refreshTaskViewsAfterPlanningMutation()
	cmds = append(cmds, defaultStatusTimeout())
	return tea.Batch(cmds...)
}

// toggleQueue adds or removes the focused task from explicit Next membership.
// Adding drops Today and normalizes nonterminal lifecycle state to Backlog;
// removing changes planning only. Because the whole move is a single durable
// ledger entry, undo/redo route through the ledger
// (undoQueueToggle/redoQueueToggle) rather than a bespoke in-memory compensation,
// so they survive a restart and honor LIFO ordering. Returns a status timeout cmd
// on success, or nil if no task is available.
func (m *AppModel) toggleQueue() tea.Cmd {
	view := m.activeView()
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

	if m.taskIsNext(task, project) {
		// Removing from Next preserves lifecycle state.
		fresh, _, err := m.queueLedgerTransition(taskID, projectID, false)
		if err != nil {
			m.err = err
			return errorCmd("%s", err)
		}
		title := taskTitleOrFallback(fresh, task)
		m.setInfoStatus(fmt.Sprintf("Removed from queue: %s", title))
	} else {
		// Adding to Next drops Today and appends one ordered membership.
		fresh, _, err := m.queueLedgerTransition(taskID, projectID, true)
		if err != nil {
			m.err = err
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

// queueLedgerTransition delegates the whole Next mutation to the application
// service, then reloads the task and planning snapshots from the committed state.
func (m *AppModel) queueLedgerTransition(taskID, projectID string, appendOrder bool) (*models.Task, *models.Project, error) {
	action := app.PlanningRemoveNext
	if appendOrder {
		action = app.PlanningAddNext
	}
	return m.applyTaskPlanningAction(taskID, projectID, action)
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
