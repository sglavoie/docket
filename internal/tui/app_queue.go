package tui

import (
	"fmt"
	"time"

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
	Task           *models.Task
	Project        *models.Project
	WasAdd         bool // true = task was added to queue
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
	m.lastToggleUndoKind = toggleUndoQueue
	m.lastQueueToggleAt = time.Now()
	m.clearToggleRedoHistory()
	m.persistFocusQueue()
	return defaultStatusTimeout()
}

// undoQueueToggle reverses the last toggleQueue action.
func (m *AppModel) undoQueueToggle() tea.Cmd {
	action := m.lastQueueToggle
	if action == nil {
		return nil
	}
	m.lastQueueToggleRedo = action
	m.lastQueueToggle = nil
	m.lastQueueToggleAt = time.Time{}
	m.lastQueueRedoAt = time.Now()
	m.lastToggleRedoKind = toggleUndoQueue
	m.lastToggleUndoKind = m.nextUndoKind(toggleUndoQueue)

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

// redoQueueToggle replays the last undone toggleQueue action.
func (m *AppModel) redoQueueToggle() tea.Cmd {
	action := m.lastQueueToggleRedo
	if action == nil {
		return nil
	}
	m.lastQueueToggleRedo = nil
	m.lastQueueRedoAt = time.Time{}
	m.lastToggleRedoKind = m.nextRedoKind(toggleUndoQueue)

	if action.WasAdd {
		// Redo add: re-add to queue.
		if !m.focusQueueIDs[action.Task.ID] {
			m.focusQueue = append(m.focusQueue, FocusTask{Task: action.Task, Project: action.Project})
			m.focusQueueIDs[action.Task.ID] = true
			m.persistFocusQueue()
		}
		// Mutual exclusivity: remove from daily plan if present.
		if action.WasInDailyPlan && m.dailyPlanIDs[action.Task.ID] {
			for i, ft := range m.dailyPlan {
				if ft.Task.ID == action.Task.ID {
					m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
					break
				}
			}
			delete(m.dailyPlanIDs, action.Task.ID)
			m.persistDailyPlan()
		}
		m.setInfoStatus(fmt.Sprintf("Redone: %s", action.Task.Title))
	} else {
		// Redo remove: re-remove from queue.
		for i, ft := range m.focusQueue {
			if ft.Task.ID == action.Task.ID {
				m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
				break
			}
		}
		delete(m.focusQueueIDs, action.Task.ID)
		m.persistFocusQueue()
		m.setInfoStatus(fmt.Sprintf("Redone: %s", action.Task.Title))
	}
	m.lastQueueToggle = action
	m.lastQueueToggleAt = time.Now()
	m.lastToggleUndoKind = toggleUndoQueue
	return defaultStatusTimeout()
}

// persistFocusQueue saves the current queue state through a checked planning
// commit. The Focus Queue is planning-domain state, so it is committed to
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
