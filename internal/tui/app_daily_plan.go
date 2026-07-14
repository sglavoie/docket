package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// dailyPlanToggleAction records a single w-key toggle for undo support.
type dailyPlanToggleAction struct {
	Task       *models.Task
	Project    *models.Project
	WasAdd     bool // true = task was added to daily plan
	WasInQueue bool // true = task was in queue before (restore on undo)
}

// toggleDailyPlan adds or removes the focused task from the daily plan.
// Returns a status timeout cmd on success, or nil if no task is available.
func (m *AppModel) toggleDailyPlan() tea.Cmd {
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
	if m.dailyPlanIDs[task.ID] {
		// Remove from daily plan.
		m.lastDailyPlanToggle = &dailyPlanToggleAction{
			Task: task, Project: project, WasAdd: false,
		}
		for i, ft := range m.dailyPlan {
			if ft.Task.ID == task.ID {
				m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
				break
			}
		}
		delete(m.dailyPlanIDs, task.ID)
		m.setInfoStatus(fmt.Sprintf("Removed from today's plan: %s", task.Title))
	} else {
		// Record before mutating so WasInQueue reflects the pre-toggle state.
		wasInQueue := m.focusQueueIDs[task.ID]
		m.lastDailyPlanToggle = &dailyPlanToggleAction{
			Task: task, Project: project, WasAdd: true, WasInQueue: wasInQueue,
		}
		// Add to daily plan.
		m.dailyPlan = append(m.dailyPlan, FocusTask{Task: task, Project: project})
		m.dailyPlanIDs[task.ID] = true
		// Mutual exclusivity: remove from queue if present.
		if wasInQueue {
			for i, ft := range m.focusQueue {
				if ft.Task.ID == task.ID {
					m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
					break
				}
			}
			delete(m.focusQueueIDs, task.ID)
			m.persistFocusQueue()
		}
		m.setInfoStatus(fmt.Sprintf("Added to today's plan: %s", task.Title))
	}
	m.lastToggleUndoKind = toggleUndoDailyPlan
	m.lastDailyPlanToggleAt = time.Now()
	m.clearToggleRedoHistory()
	m.persistDailyPlan()
	return defaultStatusTimeout()
}

// undoDailyPlanToggle reverses the last toggleDailyPlan action.
func (m *AppModel) undoDailyPlanToggle() tea.Cmd {
	action := m.lastDailyPlanToggle
	if action == nil {
		return nil
	}
	m.lastDailyPlanToggleRedo = action
	m.lastDailyPlanToggle = nil
	m.lastDailyPlanToggleAt = time.Time{}
	m.lastDailyPlanRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoDailyPlan
	m.lastToggleUndoKind = m.nextUndoKind(toggleUndoDailyPlan)

	if action.WasAdd {
		// Reverse add: remove from daily plan.
		for i, ft := range m.dailyPlan {
			if ft.Task.ID == action.Task.ID {
				m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
				break
			}
		}
		delete(m.dailyPlanIDs, action.Task.ID)
		m.persistDailyPlan()
		// Restore to queue if it was there before.
		if action.WasInQueue && !m.focusQueueIDs[action.Task.ID] {
			m.focusQueue = append(m.focusQueue, FocusTask{Task: action.Task, Project: action.Project})
			m.focusQueueIDs[action.Task.ID] = true
			m.persistFocusQueue()
		}
		m.setInfoStatus(fmt.Sprintf("Undone: %s", action.Task.Title))
	} else {
		// Reverse remove: add back to daily plan.
		if !m.dailyPlanIDs[action.Task.ID] {
			m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
			m.dailyPlanIDs[action.Task.ID] = true
			m.persistDailyPlan()
		}
		m.setInfoStatus(fmt.Sprintf("Undone: %s", action.Task.Title))
	}
	return defaultStatusTimeout()
}

// redoDailyPlanToggle replays the last undone toggleDailyPlan action.
func (m *AppModel) redoDailyPlanToggle() tea.Cmd {
	action := m.lastDailyPlanToggleRedo
	if action == nil {
		return nil
	}
	m.lastDailyPlanToggleRedo = nil
	m.lastDailyPlanRedoAt = time.Time{}
	m.lastToggleRedoKind = m.nextRedoKind(toggleUndoDailyPlan)

	if action.WasAdd {
		// Redo add: re-add to daily plan.
		if !m.dailyPlanIDs[action.Task.ID] {
			m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
			m.dailyPlanIDs[action.Task.ID] = true
			m.persistDailyPlan()
		}
		// Mutual exclusivity: remove from queue if present.
		if action.WasInQueue && m.focusQueueIDs[action.Task.ID] {
			for i, ft := range m.focusQueue {
				if ft.Task.ID == action.Task.ID {
					m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
					break
				}
			}
			delete(m.focusQueueIDs, action.Task.ID)
			m.persistFocusQueue()
		}
		m.setInfoStatus(fmt.Sprintf("Redone: %s", action.Task.Title))
	} else {
		// Redo remove: re-remove from daily plan.
		for i, ft := range m.dailyPlan {
			if ft.Task.ID == action.Task.ID {
				m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
				break
			}
		}
		delete(m.dailyPlanIDs, action.Task.ID)
		m.persistDailyPlan()
		m.setInfoStatus(fmt.Sprintf("Redone: %s", action.Task.Title))
	}
	m.lastDailyPlanToggle = action
	m.lastDailyPlanToggleAt = time.Now()
	m.lastToggleUndoKind = toggleUndoDailyPlan
	return defaultStatusTimeout()
}

// persistDailyPlan saves the current daily plan state through a checked planning
// commit. The Daily Plan is planning-domain state, so it is committed to
// planning.json alone rather than routed through a whole-config save.
func (m *AppModel) persistDailyPlan() {
	today := time.Now().Format("2006-01-02")
	items := make([]config.DailyPlanItem, len(m.dailyPlan))
	for i, ft := range m.dailyPlan {
		items[i] = config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: today,
		}
	}
	if err := m.commitPlanningMutation(func(p *config.PlanningState) {
		p.SetDailyPlan(items)
	}); err != nil {
		m.err = err
		return
	}
	if len(items) > 0 {
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.TaskPlanned = true })
	}
}
