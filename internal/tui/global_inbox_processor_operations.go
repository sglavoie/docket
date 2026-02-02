package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// openSectionPicker opens the section picker for assignment.
func (m *GlobalInboxProcessorModel) openSectionPicker() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return ShowSectionPickerMsg{
			Task:    git.Task,
			Project: git.Project,
		}
	}
}

// handleSectionPickerDone handles section picker completion.
func (m *GlobalInboxProcessorModel) handleSectionPickerDone(msg SectionPickerDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Canceled {
		return m, nil
	}

	git := m.getCurrentTask()
	if git == nil || git.Task.ID != msg.Task.ID {
		return m, nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	current := git.Project.GetTaskByID(git.Task.ID)
	if current == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return m, nil
	}

	snapshot := m.captureSnapshot(current)
	if !git.Project.MutateTask(git.Task.ID, func(t *models.Task) {
		t.Section = msg.Section
	}) {
		m.statusMessage = "Error: task no longer exists in project"
		return m, nil
	}

	sectionName := msg.Section
	if sectionName == "" {
		sectionName = "(no section)"
	}

	m.recordAction(fmt.Sprintf("section set to %s", sectionName), snapshot)
	m.statusMessage = fmt.Sprintf("Section set to '%s'", sectionName)

	// If task is leaving inbox, mark as processed
	if m.willLeaveInbox(current) {
		m.processedIDs[git.Task.ID] = true
		return m, tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
	}

	return m, m.saveCurrentProject()
}

// openTaskEditor opens the task editor.
func (m *GlobalInboxProcessorModel) openTaskEditor() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return EditTaskMsg{
			Task: git.Task,
		}
	}
}

// handleTaskUpdated handles task editor completion.
func (m *GlobalInboxProcessorModel) handleTaskUpdated(msg TaskUpdatedMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Task updated"
	return m, m.reloadCurrentProject()
}

// openDueDateInput opens the due date input.
func (m *GlobalInboxProcessorModel) openDueDateInput() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return SetDueDateMsg{
			Task:    git.Task,
			Project: git.Project,
		}
	}
}

// handleDueDateChanged handles due date input completion.
func (m *GlobalInboxProcessorModel) handleDueDateChanged(msg DueDateChangedMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Due date updated"
	return m, m.reloadCurrentProject()
}

// openDependencySelector opens the dependency selector.
func (m *GlobalInboxProcessorModel) openDependencySelector() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return ManageDependenciesMsg{
			Task:    git.Task,
			Project: git.Project,
		}
	}
}

// handleDependencySelectorDone handles dependency selector completion.
func (m *GlobalInboxProcessorModel) handleDependencySelectorDone(msg DependencySelectorDoneMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = "Dependencies updated"
	return m, m.reloadCurrentProject()
}

// openSubtaskCreator opens the task creator for subtasks.
func (m *GlobalInboxProcessorModel) openSubtaskCreator() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return CreateTaskMsg{
			ParentTask: git.Task,
		}
	}
}

// deleteCurrentTask prompts for deletion confirmation.
func (m *GlobalInboxProcessorModel) deleteCurrentTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", git.Task.ShortID)
	m.confirm.Show("delete", m.statusMessage)

	return nil
}

// executeDelete deletes the current task.
func (m *GlobalInboxProcessorModel) executeDelete() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Remove from project
	for i, t := range git.Project.Tasks {
		if t.ID == git.Task.ID {
			git.Project.Tasks = append(git.Project.Tasks[:i], git.Project.Tasks[i+1:]...)
			break
		}
	}

	// Mark as processed (removed)
	m.processedIDs[git.Task.ID] = true

	// Rebuild inbox tasks list
	m.loadAllInboxTasks()

	// Adjust index if needed
	if m.currentIndex >= len(m.inboxTasks) {
		m.currentIndex = len(m.inboxTasks) - 1
	}
	if m.currentIndex < 0 {
		m.currentIndex = 0
	}

	m.statusMessage = "Task deleted"

	return m.saveCurrentProject()
}

// openTagSelector opens the tag selector modal.
func (m *GlobalInboxProcessorModel) openTagSelector() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	return func() tea.Msg {
		return ShowTagSelectorMsg{
			Task:        git.Task,
			Project:     git.Project,
			CurrentTags: append([]string{}, git.Task.Tags...), // Copy
		}
	}
}

// handleTagSelectorDone handles tag selector completion.
func (m *GlobalInboxProcessorModel) handleTagSelectorDone(msg TagSelectorDoneMsg) (*GlobalInboxProcessorModel, tea.Cmd) {
	if msg.Canceled {
		m.statusMessage = "Tag editing canceled"
		return m, nil
	}

	git := m.getCurrentTask()
	if git == nil {
		m.statusMessage = "Error: no current task"
		return m, nil
	}
	if git.Task.ID != msg.Task.ID {
		m.statusMessage = fmt.Sprintf("Error: task ID mismatch (current: %s, msg: %s)", git.Task.ID, msg.Task.ID)
		return m, nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	current := git.Project.GetTaskByID(git.Task.ID)
	if current == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return m, nil
	}

	snapshot := m.captureSnapshot(current)
	if !git.Project.MutateTask(git.Task.ID, func(t *models.Task) {
		t.Tags = msg.SelectedTags
	}) {
		m.statusMessage = "Error: task no longer exists in project"
		return m, nil
	}

	m.recordAction("tags updated", snapshot)
	if len(msg.SelectedTags) == 0 {
		m.statusMessage = "Tags cleared"
	} else {
		m.statusMessage = fmt.Sprintf("Tags set: %s", strings.Join(msg.SelectedTags, ", "))
	}

	return m, m.saveCurrentProject()
}

// startTask starts the current task (sets status to in_progress).
func (m *GlobalInboxProcessorModel) startTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := git.Project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
		m.statusMessage = "Task must be todo or blocked to start"
		return nil
	}

	snapshot := m.captureSnapshot(task)
	task.Start()

	m.recordAction("started task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d started", task.ShortID)

	return m.saveCurrentProject()
}

// completeTask marks the current task as done.
func (m *GlobalInboxProcessorModel) completeTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	project := git.Project

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if !task.Status.IsActive() {
		m.statusMessage = "Task is not active"
		return nil
	}

	// Check for incomplete dependencies
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

	snapshot := m.captureSnapshot(task)
	task.Complete()

	m.recordAction("completed task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d completed", task.ShortID)

	// Task is leaving inbox (completed)
	m.processedIDs[task.ID] = true

	return tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
}

// blockTask blocks the current task.
func (m *GlobalInboxProcessorModel) blockTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := git.Project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if !task.Status.IsActive() || task.Status == models.StatusBlocked {
		m.statusMessage = "Task must be active and not already blocked"
		return nil
	}

	snapshot := m.captureSnapshot(task)
	task.Block()

	m.recordAction("blocked task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d blocked", task.ShortID)

	return m.saveCurrentProject()
}

// cancelTask cancels the current task.
func (m *GlobalInboxProcessorModel) cancelTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := git.Project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if !task.Status.IsActive() {
		m.statusMessage = "Task is not active"
		return nil
	}

	snapshot := m.captureSnapshot(task)
	task.Cancel()

	m.recordAction("canceled task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d canceled", task.ShortID)

	// Task is leaving inbox (canceled)
	m.processedIDs[task.ID] = true

	return tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
}

// reopenTask reopens the current task (sets status back to todo).
func (m *GlobalInboxProcessorModel) reopenTask() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
	task := git.Project.GetTaskByID(git.Task.ID)
	if task == nil {
		m.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if task.Status == models.StatusTodo {
		m.statusMessage = "Task is already todo"
		return nil
	}

	snapshot := m.captureSnapshot(task)
	task.Reopen()

	m.recordAction("reopened task", snapshot)
	m.statusMessage = fmt.Sprintf("Task #%d reopened", task.ShortID)

	return m.saveCurrentProject()
}

// sendToWaiting moves the current task to Waiting section and marks it processed.
func (m *GlobalInboxProcessorModel) sendToWaiting() tea.Cmd {
	git := m.getCurrentTask()
	if git == nil {
		return nil
	}

	// Re-lookup task to avoid stale pointer after concurrent reload
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

	// Mark as processed (it's leaving inbox)
	m.processedIDs[task.ID] = true

	return tea.Batch(m.saveCurrentProject(), m.advanceToNext(true))
}
