package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// dailyPlanToggleAction is retained as transient UI metadata while every durable
// Today mutation and its undo/redo now flow through the workspace ledger.
type dailyPlanToggleAction struct {
	Task    *models.Task
	Project *models.Project
	WasAdd  bool // true = task was added to daily plan
}

// toggleDailyPlan adds or removes the focused task from the daily plan.
// Returns a status timeout cmd on success, or nil if no task is available.
func (m *AppModel) toggleDailyPlan() tea.Cmd {
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
	add := !m.dailyPlanIDs[identity]
	fresh, _, err := m.dailyPlanLedgerTransition(taskID, projectID, add)
	if err != nil {
		m.err = err
		return errorCmd("%s", err)
	}
	if add {
		m.setInfoStatus(fmt.Sprintf("Added to today's plan: %s", taskTitleOrFallback(fresh, task)))
	} else {
		m.setInfoStatus(fmt.Sprintf("Removed from today's plan: %s", taskTitleOrFallback(fresh, task)))
	}
	m.lastToggleUndoKind = toggleUndoDailyPlan
	m.lastDailyPlanToggleAt = time.Now()
	m.clearToggleRedoHistory()
	return m.queueToggleResultCmd()
}

// dailyPlanLedgerTransition delegates both membership and any Inbox/Next
// promotion to the application service.
func (m *AppModel) dailyPlanLedgerTransition(taskID, projectID string, add bool) (*models.Task, *models.Project, error) {
	action := app.PlanningRemoveToday
	if add {
		action = app.PlanningAddToday
	}
	return m.applyTaskPlanningAction(taskID, projectID, action)
}

// undoDailyPlanToggle restores the last Today transaction through the same
// durable workspace ledger used by Next.
func (m *AppModel) undoDailyPlanToggle() tea.Cmd {
	if m.appService == nil && m.lastDailyPlanToggle != nil {
		action := m.lastDailyPlanToggle
		m.lastDailyPlanToggle = nil
		m.lastDailyPlanToggleRedo = action
		m.lastDailyPlanToggleAt = time.Time{}
		m.lastDailyPlanRedoAt = time.Now()
		m.lastToggleRedoKind = toggleUndoDailyPlan
		m.lastToggleUndoKind = m.nextUndoKind(toggleUndoDailyPlan)
		m.applyDailyPlanToggleInMemory(action, !action.WasAdd)
		return defaultStatusTimeout()
	}
	m.lastDailyPlanToggle = nil
	m.lastDailyPlanToggleAt = time.Time{}
	m.lastDailyPlanRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoDailyPlan
	m.lastToggleUndoKind = toggleUndoDailyPlan
	return m.planningLedgerRestore(false)
}

// redoDailyPlanToggle reapplies the last Today transaction through the durable
// workspace ledger.
func (m *AppModel) redoDailyPlanToggle() tea.Cmd {
	if m.appService == nil && m.lastDailyPlanToggleRedo != nil {
		action := m.lastDailyPlanToggleRedo
		m.lastDailyPlanToggleRedo = nil
		m.lastDailyPlanToggle = action
		m.lastDailyPlanRedoAt = time.Time{}
		m.lastDailyPlanToggleAt = time.Now()
		m.lastToggleUndoKind = toggleUndoDailyPlan
		m.lastToggleRedoKind = m.nextRedoKind(toggleUndoDailyPlan)
		m.applyDailyPlanToggleInMemory(action, action.WasAdd)
		return defaultStatusTimeout()
	}
	m.lastDailyPlanToggleRedo = nil
	m.lastDailyPlanRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoDailyPlan
	m.lastToggleUndoKind = toggleUndoDailyPlan
	m.lastDailyPlanToggleAt = time.Now()
	return m.planningLedgerRestore(true)
}

// applyDailyPlanToggleInMemory preserves the old isolated-model undo contract
// without creating an adapter-owned persistence path. Production models always
// use the application service and workspace ledger above.
func (m *AppModel) applyDailyPlanToggleInMemory(action *dailyPlanToggleAction, add bool) {
	if action == nil || action.Task == nil || action.Project == nil {
		return
	}
	identity := taskIdentityFor(action.Task, action.Project)
	if !add {
		m.removeFromDailyPlan(identity)
		return
	}
	if m.dailyPlanIDs == nil {
		m.dailyPlanIDs = make(map[TaskIdentity]bool)
	}
	if m.dailyPlanIDs[identity] {
		return
	}
	m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
	m.dailyPlanIDs[identity] = true
}
