package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleStatusStart sets the selected task status to in_progress (started).
func (m *KanbanViewModel) handleStatusStart() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Only start tasks that are todo or blocked
	if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
		m.statusMessage = "Cannot start: task is not todo or blocked"
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Start()
	m.statusMessage = fmt.Sprintf("Started: %s", task.Title)

	return m.saveWithStatusChange(prevStates, task)
}

// handleStatusDone sets the selected task status to done (completed).
func (m *KanbanViewModel) handleStatusDone() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Only complete active tasks
	if !task.Status.IsActive() {
		m.statusMessage = "Cannot complete: task is not active"
		return nil
	}

	// Check for incomplete dependencies
	incompleteDeps := m.Project.GetIncompleteDependencies(task)
	if len(incompleteDeps) > 0 {
		m.statusMessage = fmt.Sprintf("Cannot complete: %d incomplete dependencies", len(incompleteDeps))
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Complete()
	m.statusMessage = fmt.Sprintf("Completed: %s", task.Title)

	return m.saveWithStatusChange(prevStates, task)
}

// handleStatusBlocked sets the selected task status to blocked.
func (m *KanbanViewModel) handleStatusBlocked() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Only block active tasks that aren't already blocked
	if !task.Status.IsActive() || task.Status == models.StatusBlocked {
		m.statusMessage = "Cannot block: task is not active or already blocked"
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Block()
	m.statusMessage = fmt.Sprintf("Blocked: %s", task.Title)

	return m.saveWithStatusChange(prevStates, task)
}

// handleStatusOpen sets the selected task status to todo (reopened/open).
func (m *KanbanViewModel) handleStatusOpen() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Reopen tasks that aren't already todo
	if task.Status == models.StatusTodo {
		m.statusMessage = "Task is already open"
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Reopen()
	m.statusMessage = fmt.Sprintf("Reopened: %s", task.Title)

	return m.saveWithStatusChange(prevStates, task)
}

// handleStatusCancelled sets the selected task status to canceled.
func (m *KanbanViewModel) handleStatusCancelled() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	// Only cancel active tasks
	if !task.Status.IsActive() {
		m.statusMessage = "Cannot cancel: task is not active"
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	task.Cancel()
	m.statusMessage = fmt.Sprintf("Canceled: %s", task.Title)

	return m.saveWithStatusChange(prevStates, task)
}

// saveWithStatusChange saves the project after a status change with workflow
// rule evaluation. Returns a command that persists changes and refreshes the view.
func (m *KanbanViewModel) saveWithStatusChange(prevStates map[string]*models.Task, task *models.Task) tea.Cmd {
	// Track current position info for cursor restoration after reload
	currentSection := m.CurrentSection()
	taskID := task.ID

	// Update timestamp before save
	task.UpdatedAt = time.Now()

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
