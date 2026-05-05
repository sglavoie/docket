package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

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

// selectAllVisible selects all visible tasks in non-collapsed groups.
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
// Retained for handleBulkDelete which uses the CrossProjectTask type directly.
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

// getSelectedBulkItems returns all selected tasks as a flat slice of bulkTaskItem.
func (m *AllTasksViewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, group := range m.groups {
		for _, item := range group.Tasks {
			if m.selectedTasks[item.Task.ID] {
				items = append(items, bulkTaskItem{Task: item.Task, Project: item.Project})
			}
		}
	}
	return items
}

// finishBulk clears multiselect state, sets the status message, and refreshes.
func (m *AllTasksViewModel) finishBulk(statusMsg string) {
	m.FinishBulk(&m.statusMessage, statusMsg)
	m.loadAllTasks()
	m.clampCursor()
}

// handleBulkStatusChange applies a status change to all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	items := m.getSelectedBulkItems()
	count, err := taskOpsApplyBulk(m, items, StatusVerb(newStatus)+" Task",
		func(project *models.Project, taskID string) bool {
			task := project.GetTaskByID(taskID)
			if task == nil || task.CanTransitionTo(newStatus) != "" {
				return false
			}
			project.MutateTask(taskID, func(t *models.Task) {
				t.ApplyTransition(newStatus)
			})
			return true
		})
	if err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
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
	items := m.getSelectedBulkItems()
	count, err := taskOpsApplyBulk(m, items, "Set Priority",
		func(project *models.Project, taskID string) bool {
			task := project.GetTaskByID(taskID)
			if task == nil || task.Priority == priority {
				return false
			}
			project.MutateTask(taskID, func(t *models.Task) {
				t.Priority = priority
			})
			return true
		})
	if err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
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
	items := m.getSelectedBulkItems()
	dueDate := util.StartOfDay(time.Now().AddDate(0, 0, daysFromNow))
	count, err := taskOpsApplyBulk(m, items, "Set Due Date",
		func(project *models.Project, taskID string) bool {
			project.MutateTask(taskID, func(t *models.Task) {
				d := dueDate
				t.DueDate = &d
				t.HasTime = false
			})
			return true
		})
	if err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
		return m, nil
	}
	m.finishBulk(fmt.Sprintf("Due date %s on %d tasks", dueDateLabel(daysFromNow), count))
	return m, nil
}

// handleBulkClearDueDate clears due dates on all selected tasks.
func (m *AllTasksViewModel) handleBulkClearDueDate() (tea.Model, tea.Cmd) {
	items := m.getSelectedBulkItems()
	count, err := taskOpsApplyBulk(m, items, "Clear Due Date",
		func(project *models.Project, taskID string) bool {
			project.MutateTask(taskID, func(t *models.Task) {
				t.DueDate = nil
				t.HasTime = false
			})
			return true
		})
	if err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
		return m, nil
	}
	m.finishBulk(fmt.Sprintf("Cleared due date on %d tasks", count))
	return m, nil
}

// handleBulkSectionMove moves all selected tasks to a target section.
func (m *AllTasksViewModel) handleBulkSectionMove(targetSection string) (tea.Model, tea.Cmd) {
	items := m.getSelectedBulkItems()
	count, err := taskOpsApplyBulk(m, items, "Move to Section",
		func(project *models.Project, taskID string) bool {
			task := project.GetTaskByID(taskID)
			if task == nil || task.InSection(targetSection) {
				return false
			}
			project.MutateTask(taskID, func(t *models.Task) {
				t.SetSection(targetSection)
			})
			return true
		})
	if err != nil {
		m.setStatus(fmt.Sprintf("Error: %v", err))
		return m, nil
	}
	if count == 0 {
		m.finishBulk("No tasks were moved")
	} else {
		m.finishBulk(fmt.Sprintf("Moved %d tasks to %s", count, targetSection))
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
	allDeletedIDs := make(map[string]bool)
	for projectID, items := range byProject {
		project := m.projectCache[projectID]
		if project == nil {
			continue
		}

		if err := m.projectService.Backup(project); err != nil {
			m.setStatus(fmt.Sprintf("Failed to backup project %s: %v", project.Name, err))
			return m, nil
		}

		// Collect root IDs and expand to include all descendants
		rootIDs := make(map[string]bool, len(items))
		for _, item := range items {
			rootIDs[item.Task.ID] = true
		}
		idsToDelete := collectDescendantIDs(project.Tasks, rootIDs)
		for id := range idsToDelete {
			allDeletedIDs[id] = true
		}

		// Filter out deleted tasks and clean up dangling references
		var remaining []*models.Task
		for _, t := range project.Tasks {
			if !idsToDelete[t.ID] {
				if t.ParentID != nil && idsToDelete[*t.ParentID] {
					t.ParentID = nil
				}
				// Clear dangling DependsOn references to deleted tasks
				if len(t.DependsOn) > 0 {
					cleaned := t.DependsOn[:0]
					for _, depID := range t.DependsOn {
						if !idsToDelete[depID] {
							cleaned = append(cleaned, depID)
						}
					}
					t.DependsOn = cleaned
				}
				// Clear dangling RelatedTo references to deleted tasks
				if len(t.RelatedTo) > 0 {
					cleaned := t.RelatedTo[:0]
					for _, relID := range t.RelatedTo {
						if !idsToDelete[relID] {
							cleaned = append(cleaned, relID)
						}
					}
					t.RelatedTo = cleaned
				}
				remaining = append(remaining, t)
			}
		}
		totalDeleted += len(project.Tasks) - len(remaining)
		project.Tasks = remaining
		project.BuildTaskTree()
		if err := m.projectService.Save(project); err != nil {
			m.setStatus(fmt.Sprintf("Failed to save project %s: %v", project.Name, err))
			return m, nil
		}
	}

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	cleanupConfigTaskRefs(m.config, m.configManager, allDeletedIDs)

	m.finishBulk(fmt.Sprintf("Deleted %d tasks", totalDeleted))
	return m, nil
}
