package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *KanbanViewModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		// Clear selections when exiting
		m.selectedTasks = make(map[string]bool)
		m.statusMessage = ""
	} else {
		m.statusMessage = "Multi-select: space=toggle, a=all in column, A=all, gX=clear"
	}
}

// toggleTaskSelection toggles selection of the current task.
func (m *KanbanViewModel) toggleTaskSelection() {
	task := m.getSelectedTask()
	if task == nil {
		return
	}

	if m.selectedTasks[task.ID] {
		delete(m.selectedTasks, task.ID)
	} else {
		m.selectedTasks[task.ID] = true
	}
	m.updateSelectionStatus()
}

// selectAllInColumn selects all visible tasks in the current column.
func (m *KanbanViewModel) selectAllInColumn() {
	section := m.CurrentSection()
	visibleNodes := m.visibleNodesForSection(section)
	for _, node := range visibleNodes {
		m.selectedTasks[node.Task.ID] = true
	}
	m.updateSelectionStatus()
}

// selectAllAcrossColumns selects all visible tasks across all columns.
func (m *KanbanViewModel) selectAllAcrossColumns() {
	for _, section := range KanbanColumnOrder {
		visibleNodes := m.visibleNodesForSection(section)
		for _, node := range visibleNodes {
			m.selectedTasks[node.Task.ID] = true
		}
	}
	m.updateSelectionStatus()
}

// deselectAll clears all task selections.
func (m *KanbanViewModel) deselectAll() {
	m.selectedTasks = make(map[string]bool)
	m.updateSelectionStatus()
}

// updateSelectionStatus updates the status message with selection count.
func (m *KanbanViewModel) updateSelectionStatus() {
	count := len(m.selectedTasks)
	if count == 0 {
		m.statusMessage = "Multi-select: space=toggle, a=all in column, A=all"
	} else {
		m.statusMessage = fmt.Sprintf("%d selected (space=toggle, ,s/d/b/o=status, gX=clear)", count)
	}
}

// getSelectedTasks returns a slice of all selected tasks.
func (m *KanbanViewModel) getSelectedTasksList() []*models.Task {
	var tasks []*models.Task
	for _, task := range m.Project.Tasks {
		if m.selectedTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// isTaskSelected returns whether a task is currently selected.
func (m *KanbanViewModel) isTaskSelected(taskID string) bool {
	return m.selectedTasks[taskID]
}

// selectedCount returns the number of selected tasks.
func (m *KanbanViewModel) selectedCount() int {
	return len(m.selectedTasks)
}

// handleBulkStatusChange applies a status change to all selected tasks.
func (m *KanbanViewModel) handleBulkStatusChange(newStatus models.TaskStatus) tea.Cmd {
	tasks := m.getSelectedTasksList()
	if len(tasks) == 0 {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	changedCount := 0

	for _, task := range tasks {
		m.Project.MutateTask(task.ID, func(t *models.Task) {
			switch newStatus {
			case models.StatusInProgress:
				if t.Status == models.StatusTodo || t.Status == models.StatusBlocked {
					t.Start()
					changedCount++
				}
			case models.StatusDone:
				if t.Status.IsActive() {
					if len(m.Project.GetIncompleteDependencies(t)) == 0 {
						t.Complete()
						changedCount++
					}
				}
			case models.StatusBlocked:
				if t.Status.IsActive() && t.Status != models.StatusBlocked {
					t.Block()
					changedCount++
				}
			case models.StatusTodo:
				if t.Status != models.StatusTodo {
					t.Reopen()
					changedCount++
				}
			case models.StatusCanceled:
				if t.Status.IsActive() {
					t.Cancel()
					changedCount++
				}
			}
		})
	}

	if changedCount == 0 {
		m.statusMessage = "No tasks were changed"
		return nil
	}

	// Clear selection and exit multi-select mode
	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", changedCount, newStatus)

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
	})
}

// handleBulkSectionMove moves all selected tasks to a target section.
func (m *KanbanViewModel) handleBulkSectionMove(targetSection string) tea.Cmd {
	tasks := m.getSelectedTasksList()
	if len(tasks) == 0 {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	changedCount := 0

	for _, task := range tasks {
		if task.Section != targetSection {
			m.Project.MutateTask(task.ID, func(t *models.Task) {
				t.Section = targetSection
			})
			changedCount++
		}
	}

	if changedCount == 0 {
		m.statusMessage = "No tasks were moved"
		return nil
	}

	// Find target column index
	targetColumnIndex := 0
	for i, section := range KanbanColumnOrder {
		if section == targetSection {
			targetColumnIndex = i
			break
		}
	}

	// Clear selection and exit multi-select mode
	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Moved %d tasks to %s", changedCount, sectionDisplayName(targetSection))

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanTaskMovedMsg{
			Task:              nil,
			TargetColumnIndex: targetColumnIndex,
			TargetSection:     targetSection,
		}
	})
}

// handleBulkDelete deletes all selected tasks.
func (m *KanbanViewModel) handleBulkDelete() tea.Cmd {
	tasks := m.getSelectedTasksList()
	if len(tasks) == 0 {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)

	// Build a set of task IDs to delete
	deleteIDs := make(map[string]bool)
	for _, task := range tasks {
		deleteIDs[task.ID] = true
	}

	// Remove tasks from project
	var remaining []*models.Task
	for _, t := range m.Project.Tasks {
		if !deleteIDs[t.ID] {
			remaining = append(remaining, t)
		}
	}
	m.Project.Tasks = remaining
	m.Project.BuildTaskTree()

	// Clear selection and exit multi-select mode
	count := len(tasks)
	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Deleted %d tasks", count)

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanTaskDeletedMsg{TaskID: "", Section: m.CurrentSection()}
	})
}

// handleBulkPriorityChange applies a priority change to all selected tasks.
func (m *KanbanViewModel) handleBulkPriorityChange(priority int) tea.Cmd {
	tasks := m.getSelectedTasksList()
	if len(tasks) == 0 {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	for _, task := range tasks {
		m.Project.MutateTask(task.ID, func(t *models.Task) {
			t.Priority = priority
		})
	}

	count := len(tasks)
	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Set priority %d on %d tasks", priority, count)

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
	})
}

// handleBulkDueDateChange applies a due date change to all selected tasks.
func (m *KanbanViewModel) handleBulkDueDateChange(dueDate *time.Time, desc string) tea.Cmd {
	tasks := m.getSelectedTasksList()
	if len(tasks) == 0 {
		return nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	for _, task := range tasks {
		m.Project.MutateTask(task.ID, func(t *models.Task) {
			t.DueDate = dueDate
		})
	}

	count := len(tasks)
	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Due date %s on %d tasks", desc, count)

	return m.saveWithWorkflow(prevStates, func() tea.Msg {
		return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
	})
}
