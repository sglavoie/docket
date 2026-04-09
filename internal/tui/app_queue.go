package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// QueueToggleable is implemented by views that can provide a task for queue toggling.
type QueueToggleable interface {
	getTaskRef() (*models.Task, *models.Project)
}

// queueToggleAction records a single q-key toggle for undo support.
type queueToggleAction struct {
	Task          *models.Task
	Project       *models.Project
	WasAdd        bool // true = task was added to queue
	WasInDailyPlan bool // true = task was in daily plan before (restore on undo)
}

// toggleQueue adds or removes the focused task from the work queue.
// Returns a status timeout cmd on success, or nil if no task is available.
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
	if m.focusQueueIDs[task.ID] {
		// Remove from queue.
		m.lastQueueToggle = &queueToggleAction{
			Task: task, Project: project, WasAdd: false,
		}
		for i, ft := range m.focusQueue {
			if ft.Task.ID == task.ID {
				m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
				break
			}
		}
		delete(m.focusQueueIDs, task.ID)
		m.setInfoStatus(fmt.Sprintf("Removed from queue: %s", task.Title))
	} else {
		// Record before mutating so WasInDailyPlan reflects the pre-toggle state.
		wasInDailyPlan := m.dailyPlanIDs[task.ID]
		m.lastQueueToggle = &queueToggleAction{
			Task: task, Project: project, WasAdd: true, WasInDailyPlan: wasInDailyPlan,
		}
		// Add to queue.
		m.focusQueue = append(m.focusQueue, FocusTask{Task: task, Project: project})
		m.focusQueueIDs[task.ID] = true
		// Mutual exclusivity: remove from daily plan if present.
		if wasInDailyPlan {
			for i, ft := range m.dailyPlan {
				if ft.Task.ID == task.ID {
					m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
					break
				}
			}
			delete(m.dailyPlanIDs, task.ID)
			m.persistDailyPlan()
		}
		m.setInfoStatus(fmt.Sprintf("Added to queue: %s", task.Title))
	}
	m.persistFocusQueue()
	return defaultStatusTimeout()
}

// undoQueueToggle reverses the last toggleQueue action.
func (m *AppModel) undoQueueToggle() tea.Cmd {
	action := m.lastQueueToggle
	if action == nil {
		return nil
	}
	m.lastQueueToggle = nil

	if action.WasAdd {
		// Reverse add: remove from queue.
		for i, ft := range m.focusQueue {
			if ft.Task.ID == action.Task.ID {
				m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
				break
			}
		}
		delete(m.focusQueueIDs, action.Task.ID)
		m.persistFocusQueue()
		// Restore to daily plan if it was there before.
		if action.WasInDailyPlan && !m.dailyPlanIDs[action.Task.ID] {
			m.dailyPlan = append(m.dailyPlan, FocusTask{Task: action.Task, Project: action.Project})
			m.dailyPlanIDs[action.Task.ID] = true
			m.persistDailyPlan()
		}
		m.setInfoStatus(fmt.Sprintf("Undone: %s", action.Task.Title))
	} else {
		// Reverse remove: add back to queue.
		if !m.focusQueueIDs[action.Task.ID] {
			m.focusQueue = append(m.focusQueue, FocusTask{Task: action.Task, Project: action.Project})
			m.focusQueueIDs[action.Task.ID] = true
			m.persistFocusQueue()
		}
		m.setInfoStatus(fmt.Sprintf("Undone: %s", action.Task.Title))
	}
	return defaultStatusTimeout()
}

// persistFocusQueue saves the current queue state to config.
func (m *AppModel) persistFocusQueue() {
	if m.configManager == nil {
		return
	}
	items := make([]config.FocusQueueItem, len(m.focusQueue))
	for i, ft := range m.focusQueue {
		items[i] = config.FocusQueueItem{
			TaskID:    ft.Task.ID,
			ProjectID: ft.Project.ID,
		}
	}
	m.config.SetFocusQueue(items)
	if err := m.configManager.Save(m.config); err != nil {
		m.err = err
	}
}
