package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// inboxOpsView implementation for GlobalInboxProcessorModel.

func (m *GlobalInboxProcessorModel) getInboxTaskRef() (*models.Task, *models.Project) {
	git := m.getCurrentTask()
	if git == nil {
		return nil, nil
	}
	return git.Task, git.Project
}

func (m *GlobalInboxProcessorModel) inboxSave() tea.Cmd   { return m.saveCurrentProject() }
func (m *GlobalInboxProcessorModel) inboxReload() tea.Cmd { return m.reloadCurrentProject() }
func (m *GlobalInboxProcessorModel) inboxCaptureSnapshot(task *models.Task) *inboxSnapshot {
	return m.captureSnapshot(task)
}
func (m *GlobalInboxProcessorModel) inboxRecordAction(desc string, snapshot *inboxSnapshot) {
	m.recordAction(desc, snapshot)
}
func (m *GlobalInboxProcessorModel) setInboxStatus(msg string) { m.statusMessage = msg }
func (m *GlobalInboxProcessorModel) setInboxErr(err error)     { m.err = err }

// Delegate shared operations.

func (m *GlobalInboxProcessorModel) openSectionPicker() tea.Cmd { return inboxOpenSectionPicker(m) }
func (m *GlobalInboxProcessorModel) openTaskEditor() tea.Cmd    { return inboxOpenTaskEditor(m) }
func (m *GlobalInboxProcessorModel) openDueDateInput() tea.Cmd  { return inboxOpenDueDateInput(m) }
func (m *GlobalInboxProcessorModel) openDependencySelector() tea.Cmd {
	return inboxOpenDependencySelector(m)
}
func (m *GlobalInboxProcessorModel) openSubtaskCreator() tea.Cmd { return inboxOpenSubtaskCreator(m) }
func (m *GlobalInboxProcessorModel) openTagSelector() tea.Cmd    { return inboxOpenTagSelector(m) }

func (m *GlobalInboxProcessorModel) deleteCurrentTask() tea.Cmd {
	inboxDeletePrompt(m, &m.confirm)
	return nil
}

func (m *GlobalInboxProcessorModel) handleTaskUpdated(msg TaskUpdatedMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleTaskUpdated(m)
}

func (m *GlobalInboxProcessorModel) handleDueDateChanged(msg DueDateChangedMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleDueDateChanged(m)
}

func (m *GlobalInboxProcessorModel) handleDependencySelectorDone(msg DependencySelectorDoneMsg) (tea.Model, tea.Cmd) {
	return m, inboxHandleDependencySelectorDone(m)
}

// handleSectionPickerDone has global-specific logic: if the task leaves inbox,
// mark it as processed and advance to the next task.
func (m *GlobalInboxProcessorModel) handleSectionPickerDone(msg SectionPickerDoneMsg) (tea.Model, tea.Cmd) {
	if !inboxHandleSectionPickerCore(m, msg) {
		return m, nil
	}

	git := m.getCurrentTask()
	if git != nil {
		current := git.Project.GetTaskByID(git.Task.ID)
		if current != nil && m.willLeaveInbox(current) {
			m.processedIDs[git.Task.ID] = true
			return m, tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
		}
	}

	return m, m.saveCurrentProject()
}

func (m *GlobalInboxProcessorModel) handleTagSelectorDone(msg TagSelectorDoneMsg) (*GlobalInboxProcessorModel, tea.Cmd) {
	_, cmd := inboxHandleTagSelectorDone(m, msg)
	return m, cmd
}

// --- Global-specific operations (no local equivalent) ---

// changeTaskStatus validates and applies a status transition for the current
// global inbox task. Complete and cancel mark the task as processed and advance.
func (m *GlobalInboxProcessorModel) changeTaskStatus(newStatus models.TaskStatus, verb string) tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	project := git.Project

	task := project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if msg := task.CanTransitionTo(newStatus); msg != "" {
		m.statusMessage = msg
		return nil
	}

	if newStatus == models.StatusDone {
		incompleteDeps := project.GetIncompleteDependencies(task)
		if len(incompleteDeps) > 0 {
			depIDs := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depIDs[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			m.err = fmt.Errorf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depIDs, ", "))
			return nil
		}
	}

	snapshot := m.captureSnapshot(task)
	task.ApplyTransition(newStatus)

	m.recordAction(verb+" task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d %s", task.ShortID, verb)

	if newStatus.IsComplete() {
		m.processedIDs[task.ID] = true
		return tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
	}

	return m.saveCurrentProject()
}

// sendToWaiting moves the current task to Waiting section and marks it processed.
func (m *GlobalInboxProcessorModel) sendToWaiting() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	task := git.Project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	snapshot := m.captureSnapshot(task)
	if !git.Project.MutateTask(git.Task.ID, func(t *models.Task) {
		t.Section = models.SectionWaiting
	}) {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	m.recordAction("sent to Waiting", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d sent to Waiting", task.ShortID)

	m.processedIDs[task.ID] = true

	return tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
}

// executeDelete deletes the current task.
func (m *GlobalInboxProcessorModel) executeDelete() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	for i, t := range git.Project.Tasks {
		if t.ID == git.Task.ID {
			git.Project.Tasks = append(git.Project.Tasks[:i], git.Project.Tasks[i+1:]...)
			break
		}
	}

	m.processedIDs[git.Task.ID] = true
	m.loadAllInboxTasks()

	if m.currentIndex >= len(m.inboxTasks) {
		m.currentIndex = len(m.inboxTasks) - 1
	}
	if m.currentIndex < 0 {
		m.currentIndex = 0
	}

	m.statusMessage = "Task deleted"

	return m.saveCurrentProject()
}
