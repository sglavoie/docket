package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleKanbanStatusChange validates and applies a status transition for the
// selected kanban task. For StatusDone it also checks incomplete dependencies.
func (m *KanbanViewModel) handleKanbanStatusChange(newStatus models.TaskStatus) tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	task = m.Project.GetTaskByID(task.ID)
	if task == nil {
		return nil
	}

	if msg := task.CanTransitionTo(newStatus); msg != "" {
		m.statusMessage = msg
		return nil
	}

	// Project-level check: incomplete dependencies block completion
	if newStatus == models.StatusDone {
		if deps := m.Project.GetIncompleteDependencies(task); len(deps) > 0 {
			m.statusMessage = fmt.Sprintf("Cannot complete: %d incomplete dependencies", len(deps))
			return nil
		}
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.ApplyTransition(newStatus)
	m.statusMessage = fmt.Sprintf("Task #%d → %s", task.ShortID, newStatus)

	return m.saveWithStatusChange(prevStates, task)
}

// Convenience wrappers called from keybindings and prefix handlers.
func (m *KanbanViewModel) handleStatusStart() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusInProgress)
}
func (m *KanbanViewModel) handleStatusDone() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusDone)
}
func (m *KanbanViewModel) handleStatusBlocked() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusBlocked)
}
func (m *KanbanViewModel) handleStatusOpen() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusTodo)
}
func (m *KanbanViewModel) handleStatusCancelled() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusCanceled)
}
func (m *KanbanViewModel) handleStatusReview() tea.Cmd {
	return m.handleKanbanStatusChange(models.StatusReview)
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
