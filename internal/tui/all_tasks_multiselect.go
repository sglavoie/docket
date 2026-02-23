package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const (
	allTasksMultiSelectEnterMsg = "Multi-select: space=toggle, a=all, A=deselect, ,s/d/b=status"
	allTasksMultiSelectNoSel    = "Multi-select: space=toggle, a=all, A=deselect"
	allTasksMultiSelectWithSel  = "%d selected (space=toggle, ,s/d/b=status, D=delete)"
)

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *AllTasksViewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, allTasksMultiSelectEnterMsg)
}

// toggleTaskSelection toggles selection of the current task.
func (m *AllTasksViewModel) toggleTaskSelection() {
	item := m.getTaskAtCursor()
	if item == nil {
		return
	}
	m.ToggleAndUpdateStatus(item.Task.ID, &m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// selectAllVisible selects all visible tasks (skipping collapsed groups).
func (m *AllTasksViewModel) selectAllVisible() {
	for _, group := range m.groups {
		if m.collapsedGroups[group.Name] {
			continue
		}
		for _, item := range group.Tasks {
			m.selectedTasks[item.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// deselectAll clears all task selections.
func (m *AllTasksViewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// getSelectedItemsByProject returns all selected items grouped by project ID.
func (m *AllTasksViewModel) getSelectedItemsByProject() map[string][]CrossProjectTask {
	byProject := make(map[string][]CrossProjectTask)
	for _, group := range m.groups {
		for _, item := range group.Tasks {
			if m.selectedTasks[item.Task.ID] {
				byProject[item.Project.ID] = append(byProject[item.Project.ID], item)
			}
		}
	}
	return byProject
}

// bulkApply iterates selected tasks grouped by project, backs up each project,
// calls mutate for each item, and saves. Returns the count from mutate calls,
// or -1 if a backup failed (with statusMessage already set).
func (m *AllTasksViewModel) bulkApply(mutate func(project *models.Project, item CrossProjectTask) bool) int {
	byProject := m.getSelectedItemsByProject()
	count := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}
		if err := m.projectService.Backup(project); err != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, err)
			return -1
		}
		for _, item := range items {
			if mutate(project, item) {
				count++
			}
		}
		_ = m.projectService.Save(project)
	}
	return count
}

// finishBulk clears multiselect state, sets the status message, and refreshes.
func (m *AllTasksViewModel) finishBulk(statusMsg string) {
	m.FinishBulk(&m.statusMessage, statusMsg)
	m.loadAllTasks()
	m.clampCursor()
}

// handleBulkStatusChange applies a status change to all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	count := m.bulkApply(func(project *models.Project, item CrossProjectTask) bool {
		var changed bool
		project.MutateTask(item.Task.ID, func(t *models.Task) {
			if t.CanTransitionTo(newStatus) != "" {
				return
			}
			t.ApplyTransition(newStatus)
			changed = true
		})
		return changed
	})
	if count < 0 {
		return m, nil
	}
	if count == 0 {
		m.finishBulk("No tasks were changed")
	} else {
		m.finishBulk(fmt.Sprintf("Changed %d tasks to %s", count, newStatus))
	}
	return m, nil
}

// handleBulkPriorityChange sets the priority on all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkPriorityChange(priority int) (tea.Model, tea.Cmd) {
	count := m.bulkApply(func(project *models.Project, item CrossProjectTask) bool {
		if item.Task.Priority == priority {
			return false
		}
		project.MutateTask(item.Task.ID, func(t *models.Task) {
			t.Priority = priority
		})
		return true
	})
	if count < 0 {
		return m, nil
	}
	if count == 0 {
		m.finishBulk("No tasks were changed")
	} else {
		m.finishBulk(fmt.Sprintf("Set priority %d on %d tasks", priority, count))
	}
	return m, nil
}

// handleBulkDueDateChange sets a due date on all selected tasks.
func (m *AllTasksViewModel) handleBulkDueDateChange(daysFromNow int) (tea.Model, tea.Cmd) {
	dueDate := time.Now().AddDate(0, 0, daysFromNow)
	dueDate = util.StartOfDay(dueDate)

	count := m.bulkApply(func(project *models.Project, item CrossProjectTask) bool {
		project.MutateTask(item.Task.ID, func(t *models.Task) {
			t.DueDate = &dueDate
		})
		return true
	})
	if count < 0 {
		return m, nil
	}
	m.finishBulk(fmt.Sprintf("Due date %s on %d tasks", dueDateLabel(daysFromNow), count))
	return m, nil
}

// handleBulkClearDueDate clears due dates on all selected tasks.
func (m *AllTasksViewModel) handleBulkClearDueDate() (tea.Model, tea.Cmd) {
	count := m.bulkApply(func(project *models.Project, item CrossProjectTask) bool {
		project.MutateTask(item.Task.ID, func(t *models.Task) {
			t.DueDate = nil
		})
		return true
	})
	if count < 0 {
		return m, nil
	}
	m.finishBulk(fmt.Sprintf("Cleared due date on %d tasks", count))
	return m, nil
}

// handleBulkSectionMove moves all selected tasks to a target section.
func (m *AllTasksViewModel) handleBulkSectionMove(targetSection string) (tea.Model, tea.Cmd) {
	count := m.bulkApply(func(project *models.Project, item CrossProjectTask) bool {
		if item.Task.Section == targetSection {
			return false
		}
		project.MutateTask(item.Task.ID, func(t *models.Task) {
			t.Section = targetSection
		})
		return true
	})
	if count < 0 {
		return m, nil
	}
	if count == 0 {
		m.finishBulk("No tasks were moved")
	} else {
		m.finishBulk(fmt.Sprintf("Moved %d tasks to %s", count, sectionDisplayName(targetSection)))
	}
	return m, nil
}

// handleBulkDelete deletes all selected tasks.
func (m *AllTasksViewModel) handleBulkDelete() (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	totalDeleted := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		if err := m.projectService.Backup(project); err != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, err)
			return m, nil
		}
		deleteIDs := make(map[string]bool)
		for _, item := range items {
			deleteIDs[item.Task.ID] = true
		}

		var remaining []*models.Task
		for _, t := range project.Tasks {
			if !deleteIDs[t.ID] {
				remaining = append(remaining, t)
			}
		}
		totalDeleted += len(project.Tasks) - len(remaining)
		project.Tasks = remaining
		project.BuildTaskTree()
		_ = m.projectService.Save(project)
	}

	m.finishBulk(fmt.Sprintf("Deleted %d tasks", totalDeleted))
	return m, nil
}
