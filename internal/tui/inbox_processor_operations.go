package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// openSectionPicker opens the section picker for assignment.
func (m *InboxProcessorModel) openSectionPicker() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return ShowSectionPickerMsg{
			Task:    task,
			Project: m.project,
		}
	}
}

// handleSectionPickerDone handles section picker completion.
func (m *InboxProcessorModel) handleSectionPickerDone(msg SectionPickerDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Canceled {
		return m, nil
	}

	task := m.getCurrentTask()
	if task == nil || task.ID != msg.Task.ID {
		return m, nil
	}

	snapshot := m.captureSnapshot(task)
	task.Section = msg.Section
	task.UpdatedAt = time.Now()

	sectionName := msg.Section
	if sectionName == "" {
		sectionName = "(no section)"
	}

	m.recordAction(fmt.Sprintf("section set to %s", sectionName), snapshot)
	m.statusMessage = fmt.Sprintf("Section set to '%s'", sectionName)

	return m, m.saveProject()
}

// openTaskEditor opens the task editor.
func (m *InboxProcessorModel) openTaskEditor() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return EditTaskMsg{
			Task: task,
		}
	}
}

// handleTaskUpdated handles task editor completion.
func (m *InboxProcessorModel) handleTaskUpdated(msg TaskUpdatedMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Task updated"
	return m, m.reloadProject()
}

// openDueDateInput opens the due date input.
func (m *InboxProcessorModel) openDueDateInput() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return SetDueDateMsg{
			Task:    task,
			Project: m.project,
		}
	}
}

// handleDueDateChanged handles due date input completion.
func (m *InboxProcessorModel) handleDueDateChanged(msg DueDateChangedMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Due date updated"
	return m, m.reloadProject()
}

// openDependencySelector opens the dependency selector.
func (m *InboxProcessorModel) openDependencySelector() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return ManageDependenciesMsg{
			Task:    task,
			Project: m.project,
		}
	}
}

// handleDependencySelectorDone handles dependency selector completion.
func (m *InboxProcessorModel) handleDependencySelectorDone(msg DependencySelectorDoneMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Dependencies updated"
	return m, m.reloadProject()
}

// openSubtaskCreator opens the task creator for subtasks.
func (m *InboxProcessorModel) openSubtaskCreator() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return CreateTaskMsg{
			ParentTask: task,
		}
	}
}

// deleteCurrentTask prompts for deletion confirmation.
func (m *InboxProcessorModel) deleteCurrentTask() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", task.ShortID)
	m.confirm.Show("delete", m.statusMessage)

	return nil
}

// executeDelete deletes the current task.
func (m *InboxProcessorModel) executeDelete() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	// Remove from project
	for i, t := range m.project.Tasks {
		if t.ID == task.ID {
			m.project.Tasks = append(m.project.Tasks[:i], m.project.Tasks[i+1:]...)
			break
		}
	}

	// Remove from inbox tasks list
	m.inboxTasks = getInboxTasksForModel(m.project)

	// Adjust index if needed
	if m.currentIndex >= len(m.inboxTasks) {
		m.currentIndex = len(m.inboxTasks) - 1
	}
	if m.currentIndex < 0 {
		m.currentIndex = 0
	}

	m.statusMessage = "Task deleted"

	return m.saveProject()
}

// openTagSelector opens the tag selector modal.
func (m *InboxProcessorModel) openTagSelector() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	return func() tea.Msg {
		return ShowTagSelectorMsg{
			Task:        task,
			Project:     m.project,
			CurrentTags: append([]string{}, task.Tags...), // Copy
		}
	}
}

// handleTagSelectorDone handles tag selector completion.
func (m *InboxProcessorModel) handleTagSelectorDone(msg TagSelectorDoneMsg) (*InboxProcessorModel, tea.Cmd) {
	if msg.Canceled {
		m.statusMessage = "Tag editing canceled"
		return m, nil
	}

	task := m.getCurrentTask()
	if task == nil {
		m.statusMessage = "Error: no current task"
		return m, nil
	}
	if task.ID != msg.Task.ID {
		m.statusMessage = fmt.Sprintf("Error: task ID mismatch (current: %s, msg: %s)", task.ID, msg.Task.ID)
		return m, nil
	}

	snapshot := m.captureSnapshot(task)
	task.Tags = msg.SelectedTags
	task.UpdatedAt = time.Now()

	m.recordAction("tags updated", snapshot)
	if len(msg.SelectedTags) == 0 {
		m.statusMessage = "Tags cleared"
	} else {
		m.statusMessage = fmt.Sprintf("Tags set: %s", strings.Join(msg.SelectedTags, ", "))
	}

	return m, m.saveProject()
}
