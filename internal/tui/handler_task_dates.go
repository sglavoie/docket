package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskDateMessages handles task date-related messages (due dates, snooze).
// Returns true if the message was handled.
func (m *AppModel) handleTaskDateMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case SetDueDateMsg:
		return m.handleSetDueDate(msg)

	case SetSnoozeDateMsg:
		return m.handleSetSnoozeDate(msg)

	case SnoozeDateChangedMsg:
		return m.handleSnoozeDateChanged(msg)

	case DueDateChangedMsg:
		return m.handleDueDateChanged(msg)
	}
	return false, nil
}

// handleSetDueDate handles the SetDueDateMsg.
func (m *AppModel) handleSetDueDate(msg SetDueDateMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.showView(ViewDateInput, NewDateInputModel(msg.Task, msg.Project))
	}
	return true, nil
}

// handleSetSnoozeDate handles the SetSnoozeDateMsg.
func (m *AppModel) handleSetSnoozeDate(msg SetSnoozeDateMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.showView(ViewSnoozeInput, NewSnoozeInputModel(msg.Task, msg.Project))
	}
	return true, nil
}

// handleSnoozeDateChanged handles the SnoozeDateChangedMsg.
func (m *AppModel) handleSnoozeDateChanged(msg SnoozeDateChangedMsg) (bool, tea.Cmd) {
	m.popView()
	return true, nil
}

// handleDueDateChanged handles the DueDateChangedMsg, including bulk operations.
func (m *AppModel) handleDueDateChanged(msg DueDateChangedMsg) (bool, tea.Cmd) {
	m.popView()

	// Check if this is a bulk operation from palette
	if len(m.pendingBulkTasks) > 1 && msg.Task != nil {
		// The first task was already saved by DateInputModel
		// Apply the same date to remaining tasks
		newDate := msg.Task.DueDate
		newHasTime := msg.Task.HasTime
		count := 1 // First task already updated
		projectMap := make(map[string]*models.Project)

		for i, item := range m.pendingBulkTasks {
			if i == 0 {
				// Skip the first task (already saved by DateInputModel)
				projectMap[item.Project.ID] = item.Project
				continue
			}
			if item.Task != nil && item.Project != nil {
				if item.Project.MutateTask(item.Task.ID, func(t *models.Task) {
					if newDate != nil {
						dateCopy := *newDate
						t.DueDate = &dateCopy
					} else {
						t.DueDate = nil
					}
					t.HasTime = newHasTime
				}) {
					projectMap[item.Project.ID] = item.Project
					count++
				}
			}
		}

		// Build list of projects to save (except first which was already saved)
		var projectsToSave []*models.Project
		for _, project := range projectMap {
			if project.ID != m.pendingBulkTasks[0].Project.ID {
				projectsToSave = append(projectsToSave, project)
			}
		}

		var successMsg string
		if newDate != nil {
			successMsg = fmt.Sprintf("Set due date to %s for %d tasks", newDate.Format("2006-01-02"), count)
		} else {
			successMsg = fmt.Sprintf("Cleared due date for %d tasks", count)
		}
		m.pendingBulkTasks = nil

		if len(projectsToSave) > 0 {
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveMultipleProjectsAsync(projectsToSave, successMsg),
			)
		}
		m.setSuccessStatus(successMsg)
		m.refreshAllViews()
		return true, defaultStatusTimeout()
	}

	m.pendingBulkTasks = nil
	m.refreshAllViews()
	return true, nil
}
