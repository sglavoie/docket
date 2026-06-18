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
	groups := groupTaskProjectRefsByProject(m.getSelectedBulkItems())
	if len(groups) == 0 {
		return m, nil
	}

	totalDeleted := 0
	allDeletedIDs := make(map[string]bool)
	var changedGroups []changedTaskProjectGroup

	for _, group := range groups {
		project := group.Project
		if project == nil {
			continue
		}

		// Collect root IDs and expand to include all descendants
		rootIDs := make(map[string]bool, len(group.Items))
		for _, item := range group.Items {
			rootIDs[item.Task.ID] = true
		}
		idsToDelete := collectDescendantIDs(project.Tasks, rootIDs)
		for id := range idsToDelete {
			allDeletedIDs[id] = true
		}

		prevStates := captureTaskStates(project.Tasks)
		beforeCount := len(project.Tasks)
		deleteTasksFromProject(project, idsToDelete)
		deletedCount := beforeCount - len(project.Tasks)
		if deletedCount == 0 {
			continue
		}
		totalDeleted += deletedCount
		changedGroups = append(changedGroups, changedTaskProjectGroup{
			group:      group,
			taskIDs:    sortedTaskIDsFromSet(idsToDelete),
			prevStates: prevStates,
		})
	}

	groupID, groupProjectIDs := workflowGroupMetadata(taskProjectGroupsFromChanged(changedGroups))
	for _, changed := range changedGroups {
		opts := BuildWorkflowOpts(
			m.projectService,
			changed.group.Project,
			m.config,
			m.configManager,
			changed.prevStates,
			"Delete Tasks",
			fmt.Sprintf("%d tasks", len(changed.taskIDs)),
			changed.taskIDs,
		)
		opts.UndoGroupID = groupID
		opts.UndoGroupProjectIDs = groupProjectIDs
		if err := persistWithWorkflow(opts); err != nil {
			m.setStatus(fmt.Sprintf("Failed to save project %s: %v", changed.group.Project.Name, err))
			return m, nil
		}
	}

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	cleanupConfigTaskRefs(m.config, m.configManager, allDeletedIDs)

	m.finishBulk(fmt.Sprintf("Deleted %d tasks", totalDeleted))
	return m, nil
}
