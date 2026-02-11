package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *AllTasksViewModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.selectedTasks = make(map[string]bool)
		m.statusMessage = ""
	} else {
		m.statusMessage = "Multi-select: space=toggle, a=all, A=deselect, ,s/d/b=status"
	}
}

// toggleTaskSelection toggles selection of the current task.
func (m *AllTasksViewModel) toggleTaskSelection() {
	item := m.getTaskAtCursor()
	if item == nil {
		return
	}

	if m.selectedTasks[item.Task.ID] {
		delete(m.selectedTasks, item.Task.ID)
	} else {
		m.selectedTasks[item.Task.ID] = true
	}
	m.updateSelectionStatus()
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
	m.updateSelectionStatus()
}

// deselectAll clears all task selections.
func (m *AllTasksViewModel) deselectAll() {
	m.selectedTasks = make(map[string]bool)
	m.updateSelectionStatus()
}

// updateSelectionStatus updates the status message with selection count.
func (m *AllTasksViewModel) updateSelectionStatus() {
	count := m.selectedCount()
	if count == 0 {
		m.statusMessage = "Multi-select: space=toggle, a=all, A=deselect"
	} else {
		m.statusMessage = fmt.Sprintf("%d selected (space=toggle, ,s/d/b=status, D=delete)", count)
	}
}

// selectedCount returns the number of selected tasks.
func (m *AllTasksViewModel) selectedCount() int {
	return len(m.selectedTasks)
}

// getSelectedItemsByProject returns all selected items grouped by project ID.
func (m *AllTasksViewModel) getSelectedItemsByProject() map[string][]AllTasksItem {
	byProject := make(map[string][]AllTasksItem)
	for _, group := range m.groups {
		for _, item := range group.Tasks {
			if m.selectedTasks[item.Task.ID] {
				byProject[item.Project.ID] = append(byProject[item.Project.ID], item)
			}
		}
	}
	return byProject
}

// handleBulkStatusChange applies a status change to all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	changedCount := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
			return m, nil
		}
		for _, item := range items {
			project.MutateTask(item.Task.ID, func(t *models.Task) {
				switch newStatus {
				case models.StatusInProgress:
					if t.Status == models.StatusTodo || t.Status == models.StatusBlocked {
						t.Start()
						changedCount++
					}
				case models.StatusDone:
					if t.Status.IsActive() {
						t.Complete()
						changedCount++
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
				case models.StatusReview:
					if t.Status.IsActive() && t.Status != models.StatusReview {
						t.Review()
						changedCount++
					}
				}
			})
		}
		_ = m.projectService.Save(project)
	}

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false

	if changedCount == 0 {
		m.statusMessage = "No tasks were changed"
	} else {
		m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", changedCount, newStatus)
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, nil
}

// handleBulkPriorityChange sets the priority on all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkPriorityChange(priority int) (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	count := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
			return m, nil
		}
		for _, item := range items {
			if item.Task.Priority != priority {
				project.MutateTask(item.Task.ID, func(t *models.Task) {
					t.Priority = priority
				})
				count++
			}
		}
		_ = m.projectService.Save(project)
	}

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false

	if count == 0 {
		m.statusMessage = "No tasks were changed"
	} else {
		m.statusMessage = fmt.Sprintf("Set priority %d on %d tasks", priority, count)
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, nil
}

// handleBulkDueDateChange sets a due date on all selected tasks.
func (m *AllTasksViewModel) handleBulkDueDateChange(daysFromNow int) (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	dueDate := time.Now().AddDate(0, 0, daysFromNow)
	dueDate = time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, dueDate.Location())
	count := 0

	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
			return m, nil
		}
		for _, item := range items {
			project.MutateTask(item.Task.ID, func(t *models.Task) {
				t.DueDate = &dueDate
				count++
			})
		}
		_ = m.projectService.Save(project)
	}

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Due date %s on %d tasks", dueDateLabel(daysFromNow), count)
	m.loadAllTasks()
	m.clampCursor()
	return m, nil
}

// handleBulkClearDueDate clears due dates on all selected tasks.
func (m *AllTasksViewModel) handleBulkClearDueDate() (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	count := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
			return m, nil
		}
		for _, item := range items {
			project.MutateTask(item.Task.ID, func(t *models.Task) {
				t.DueDate = nil
				count++
			})
		}
		_ = m.projectService.Save(project)
	}

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Cleared due date on %d tasks", count)
	m.loadAllTasks()
	m.clampCursor()
	return m, nil
}

// handleBulkSectionMove moves all selected tasks to a target section.
func (m *AllTasksViewModel) handleBulkSectionMove(targetSection string) (tea.Model, tea.Cmd) {
	byProject := m.getSelectedItemsByProject()
	if len(byProject) == 0 {
		return m, nil
	}

	count := 0
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
			return m, nil
		}
		for _, item := range items {
			if item.Task.Section != targetSection {
				project.MutateTask(item.Task.ID, func(t *models.Task) {
					t.Section = targetSection
				})
				count++
			}
		}
		_ = m.projectService.Save(project)
	}

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Moved %d tasks to %s", count, sectionDisplayName(targetSection))
	m.loadAllTasks()
	m.clampCursor()
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

		errBackup := m.projectService.Backup(project)
		if errBackup != nil {
			m.statusMessage = fmt.Sprintf("Failed to backup project %s: %v", project.Name, errBackup)
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

	m.selectedTasks = make(map[string]bool)
	m.multiSelectMode = false
	m.statusMessage = fmt.Sprintf("Deleted %d tasks", totalDeleted)
	m.loadAllTasks()
	m.clampCursor()
	return m, nil
}
