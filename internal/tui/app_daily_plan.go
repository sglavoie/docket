package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

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
	m.lastTodayToggleAt = time.Time{}
	m.lastTodayRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	return m.planningLedgerRestore(false)
}

// redoTodayToggle reapplies the last Today transaction through the durable
// workspace ledger.
func (m *AppModel) redoTodayToggle() tea.Cmd {
	m.lastTodayRedoAt = time.Time{}
	m.lastToggleRedoKind = toggleUndoToday
	m.lastToggleUndoKind = toggleUndoToday
	m.lastTodayToggleAt = time.Now()
	return m.planningLedgerRestore(true)
}
