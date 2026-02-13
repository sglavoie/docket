package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleStatusTransition validates and applies a status transition for the
// selected kanban task using the shared applyStatusChange function.
func (m *KanbanViewModel) handleStatusTransition(status models.TaskStatus) tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	result := applyStatusChange(m.workflowPersistFn(), m.Project, task.ID, status)
	if result.Err != nil {
		m.statusMessage = result.Err.Error()
		return nil
	}
	if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	return result.Cmd
}

// saveWithStatusChange saves the project after a status change with workflow
// rule evaluation. Returns a command that persists changes and refreshes the view.
func (m *KanbanViewModel) saveWithStatusChange(prevStates map[string]*models.Task, task *models.Task) tea.Cmd {
	// Track current position info for cursor restoration after reload
	currentSection := m.CurrentSection()
	taskID := task.ID

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanStatusChangedMsg{
			TaskID:          taskID,
			PreviousSection: currentSection,
		}
	})
}

// handleEditTask opens the selected task in the configured editor.
// Returns an EditTaskMsg that the app will handle to launch the editor.
func (m *KanbanViewModel) handleEditTask() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return EditTaskMsg{Task: task}
	}
}
