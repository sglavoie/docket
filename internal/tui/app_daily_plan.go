package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// todayToggleAction is retained as transient UI metadata while every durable
// Today mutation and its undo/redo now flow through the workspace ledger.
type todayToggleAction struct {
	Task    *models.Task
	Project *models.Project
	WasAdd  bool // true = task was added to daily plan
}

// toggleToday adds or removes the focused task from the daily plan.
// Returns a status timeout cmd on success, or nil if no task is available.
func (m *AppModel) toggleToday() tea.Cmd {
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
	identity := taskIdentityFor(task, project)
	add := !m.todayIDs[identity]
	fresh, _, err := m.todayLedgerTransition(taskID, projectID, add)
	if err != nil {
		m.err = err
		return errorCmd("%s", err)
	}
	if add {
		m.setInfoStatus(fmt.Sprintf("Added to today's plan: %s", taskTitleOrFallback(fresh, task)))
	} else {
		m.setInfoStatus(fmt.Sprintf("Removed from today's plan: %s", taskTitleOrFallback(fresh, task)))
	}
	m.lastToggleUndoKind = toggleUndoToday
	m.lastTodayToggleAt = time.Now()
	m.clearToggleRedoHistory()
	return m.queueToggleResultCmd()
}

// todayLedgerTransition delegates both membership and any Inbox/Next
// promotion to the application service.
func (m *AppModel) todayLedgerTransition(taskID, projectID string, add bool) (*models.Task, *models.Project, error) {
	action := app.PlanningRemoveToday
	if add {
		action = app.PlanningAddToday
	}
	return m.applyTaskPlanningAction(taskID, projectID, action)
}

// undoTodayToggle restores the last Today transaction through the same
// durable workspace ledger used by Next.
func (m *AppModel) undoTodayToggle() tea.Cmd {
	if m.appService == nil && m.lastTodayToggle != nil {
		action := m.lastTodayToggle
		m.lastTodayToggle = nil
		m.lastTodayToggleRedo = action
		m.lastTodayToggleAt = time.Time{}
		m.lastTodayRedoAt = time.Now()
		m.lastToggleRedoKind = toggleUndoToday
		m.lastToggleUndoKind = m.nextUndoKind(toggleUndoToday)
		m.applyTodayToggleInMemory(action, !action.WasAdd)
		return defaultStatusTimeout()
	}
	m.lastTodayToggle = nil
	m.lastTodayToggleAt = time.Time{}
	m.lastTodayRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	return m.planningLedgerRestore(false)
}

// redoTodayToggle reapplies the last Today transaction through the durable
// workspace ledger.
func (m *AppModel) redoTodayToggle() tea.Cmd {
	if m.appService == nil && m.lastTodayToggleRedo != nil {
		action := m.lastTodayToggleRedo
		m.lastTodayToggleRedo = nil
		m.lastTodayToggle = action
		m.lastTodayRedoAt = time.Time{}
		m.lastTodayToggleAt = time.Now()
		m.lastToggleUndoKind = toggleUndoToday
		m.lastToggleRedoKind = m.nextRedoKind(toggleUndoToday)
		m.applyTodayToggleInMemory(action, action.WasAdd)
		return defaultStatusTimeout()
	}
	m.lastTodayToggleRedo = nil
	m.lastTodayRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	m.lastTodayToggleAt = time.Now()
	return m.planningLedgerRestore(true)
}

// applyTodayToggleInMemory preserves the old isolated-model undo contract
// without creating an adapter-owned persistence path. Production models always
// use the application service and workspace ledger above.
func (m *AppModel) applyTodayToggleInMemory(action *todayToggleAction, add bool) {
	if action == nil || action.Task == nil || action.Project == nil {
		return
	}
	identity := taskIdentityFor(action.Task, action.Project)
	if !add {
		m.removeFromToday(identity)
		return
	}
	if m.todayIDs == nil {
		m.todayIDs = make(map[TaskIdentity]bool)
	}
	if m.todayIDs[identity] {
		return
	}
	m.today = append(m.today, PlanningTask{Task: action.Task, Project: action.Project})
	m.todayIDs[identity] = true
}
