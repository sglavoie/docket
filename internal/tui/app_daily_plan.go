package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

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
		for i, ft := range m.dailyPlan {
			if ft.Task.ID == task.ID {
				m.dailyPlan = append(m.dailyPlan[:i], m.dailyPlan[i+1:]...)
				break
			}
		}
		delete(m.dailyPlanIDs, task.ID)
		m.setInfoStatus(fmt.Sprintf("Removed from today's plan: %s", task.Title))
	} else {
		// Add to daily plan.
		m.dailyPlan = append(m.dailyPlan, FocusTask{Task: task, Project: project})
		m.dailyPlanIDs[task.ID] = true
		// Mutual exclusivity: remove from queue if present.
		if m.focusQueueIDs[task.ID] {
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
	m.persistDailyPlan()
	return defaultStatusTimeout()
}

// persistDailyPlan saves the current daily plan state to config.
func (m *AppModel) persistDailyPlan() {
	if m.configManager == nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	items := make([]config.DailyPlanItem, len(m.dailyPlan))
	for i, ft := range m.dailyPlan {
		items[i] = config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: today,
		}
	}
	m.config.SetDailyPlan(items)
	if err := m.configManager.Save(m.config); err != nil {
		m.err = err
	}
}
