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
		for i, ft := range m.focusQueue {
			if ft.Task.ID == task.ID {
				m.focusQueue = append(m.focusQueue[:i], m.focusQueue[i+1:]...)
				break
			}
		}
		delete(m.focusQueueIDs, task.ID)
		m.setInfoStatus(fmt.Sprintf("Removed from queue: %s", task.Title))
	} else {
		// Add to queue.
		m.focusQueue = append(m.focusQueue, FocusTask{Task: task, Project: project})
		m.focusQueueIDs[task.ID] = true
		// Mutual exclusivity: remove from daily plan if present.
		if m.dailyPlanIDs[task.ID] {
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
