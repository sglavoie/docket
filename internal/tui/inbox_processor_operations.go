package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// inboxOpsView implementation for InboxProcessorModel.

func (m *InboxProcessorModel) getInboxTaskRef() (*models.Task, *models.Project) {
	task := m.getCurrentTask()
	if task == nil {
		return nil, nil
	}
	return task, m.project
}

func (m *InboxProcessorModel) inboxSave() tea.Cmd   { return m.saveProject() }
func (m *InboxProcessorModel) inboxReload() tea.Cmd { return m.reloadProject() }
func (m *InboxProcessorModel) inboxCaptureSnapshot(task *models.Task) *inboxSnapshot {
	return m.captureSnapshot(task)
}
func (m *InboxProcessorModel) inboxRecordAction(desc string, snapshot *inboxSnapshot) {
	m.recordAction(desc, snapshot)
}
func (m *InboxProcessorModel) setInboxStatus(msg string) { m.statusMessage = msg }

// Delegate shared operations.

func (m *InboxProcessorModel) openSectionPicker() tea.Cmd      { return inboxOpenSectionPicker(m) }
func (m *InboxProcessorModel) openTaskEditor() tea.Cmd         { return inboxOpenTaskEditor(m) }
func (m *InboxProcessorModel) openDueDateInput() tea.Cmd       { return inboxOpenDueDateInput(m) }
func (m *InboxProcessorModel) openDependencySelector() tea.Cmd { return inboxOpenDependencySelector(m) }
func (m *InboxProcessorModel) openSubtaskCreator() tea.Cmd     { return inboxOpenSubtaskCreator(m) }
func (m *InboxProcessorModel) openTagSelector() tea.Cmd        { return inboxOpenTagSelector(m) }

func (m *InboxProcessorModel) deleteCurrentTask() tea.Cmd {
	inboxDeletePrompt(m, &m.confirm)
	return nil
}

func (m *InboxProcessorModel) handleTaskUpdated(msg TaskUpdatedMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleTaskUpdated(m)
}

func (m *InboxProcessorModel) handleDueDateChanged(msg DueDateChangedMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleDueDateChanged(m)
}

func (m *InboxProcessorModel) handleDependencySelectorDone(msg DependencySelectorDoneMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleDependencySelectorDone(m)
}

func (m *InboxProcessorModel) handleSectionPickerDone(msg SectionPickerDoneMsg) (tea.Model, tea.Cmd) {
	if !inboxHandleSectionPickerCore(m, msg) {
		return m, nil
	}
	return m, m.saveProject()
}

func (m *InboxProcessorModel) handleTagSelectorDone(msg TagSelectorDoneMsg) (*InboxProcessorModel, tea.Cmd) {
	_, cmd := inboxHandleTagSelectorDone(m, msg)
	return m, cmd
}

// executeDelete deletes the current task.
func (m *InboxProcessorModel) executeDelete() tea.Cmd {
	task := m.getCurrentTask()
	if task == nil {
		return nil
	}

	for i, t := range m.project.Tasks {
		if t.ID == task.ID {
			m.project.Tasks = append(m.project.Tasks[:i], m.project.Tasks[i+1:]...)
			break
		}
	}

	m.inboxTasks = getInboxTasksForModel(m.project)

	if m.currentIndex >= len(m.inboxTasks) {
		m.currentIndex = len(m.inboxTasks) - 1
	}
	if m.currentIndex < 0 {
		m.currentIndex = 0
	}

	m.statusMessage = "Task deleted"

	return m.saveProject()
}
