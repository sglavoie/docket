package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// bulkTaskItem represents a task+project pair for bulk operations.
type bulkTaskItem struct {
	Task    *models.Task
	Project *models.Project
}

// bulkProjectGroup groups bulk items belonging to a single project.
type bulkProjectGroup struct {
	Project *models.Project
	Items   []bulkTaskItem
}

// groupBulkItemsByProject groups items by project, preserving insertion order.
func groupBulkItemsByProject(items []bulkTaskItem) []bulkProjectGroup {
	seen := make(map[string]int) // projectID → index in result
	var result []bulkProjectGroup
	for _, item := range items {
		idx, ok := seen[item.Project.ID]
		if !ok {
			idx = len(result)
			seen[item.Project.ID] = idx
			result = append(result, bulkProjectGroup{Project: item.Project})
		}
		result[idx].Items = append(result[idx].Items, item)
	}
	return result
}

// taskOpsApplyBulk runs a bulk operation with per-project workflow pipeline.
// Records undo snapshots, runs auto-unblock/recurring/rules per project.
// Returns count of successfully mutated tasks.
func taskOpsApplyBulk(
	v taskOpsView,
	items []bulkTaskItem,
	undoAction string,
	mutate func(project *models.Project, taskID string) bool,
) (int, error) {
	byProject := groupBulkItemsByProject(items)
	total := 0

	for _, group := range byProject {
		project := group.Project
		taskIDs := make([]string, 0, len(group.Items))
		for _, item := range group.Items {
			taskIDs = append(taskIDs, item.Task.ID)
		}

		for _, item := range group.Items {
			if mutate(project, item.Task.ID) {
				total++
			}
		}

		undoCtx := fmt.Sprintf("%d tasks", len(group.Items))
		opts := BuildWorkflowOpts(
			v.getPersister(), project, v.getConfig(), v.getConfigManager(),
			nil, undoAction, undoCtx, taskIDs,
		)
		if err := persistWithWorkflow(opts); err != nil {
			return total, err
		}
	}

	v.reloadData()
	v.clampCursor()
	return total, nil
}

// dispatchBulkPrefix handles prefix dispatch when multi-select is active.
// Returns (model, cmd, handled).
func dispatchBulkPrefix(prefix, key string, v taskOpsView, items []bulkTaskItem) (tea.Model, tea.Cmd, bool) {
	if len(items) == 0 {
		return nil, nil, false
	}

	switch prefix {
	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			count, err := taskOpsApplyBulk(v, items, StatusVerb(status)+" Task",
				func(project *models.Project, taskID string) bool {
					task := project.GetTaskByID(taskID)
					if task == nil || task.CanTransitionTo(status) != "" {
						return false
					}
					project.MutateTask(taskID, func(t *models.Task) {
						t.ApplyTransition(status)
					})
					return true
				})
			if err != nil {
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			if count == 0 {
				v.setStatus("No tasks were changed")
			} else {
				v.setStatus(fmt.Sprintf("Changed %d tasks to %s", count, status))
			}
			return v, nil, true
		}

	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			count, err := taskOpsApplyBulk(v, items, "Set Priority",
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
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			if count == 0 {
				v.setStatus("No tasks were changed")
			} else {
				v.setStatus(fmt.Sprintf("Set priority %d on %d tasks", priority, count))
			}
			return v, nil, true
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if isClear {
				count, err := taskOpsApplyBulk(v, items, "Clear Due Date",
					func(project *models.Project, taskID string) bool {
						project.MutateTask(taskID, func(t *models.Task) {
							t.DueDate = nil
							t.HasTime = false
						})
						return true
					})
				if err != nil {
					v.setStatus(fmt.Sprintf("Error: %v", err))
					return v, nil, true
				}
				v.setStatus(fmt.Sprintf("Cleared due date on %d tasks", count))
				return v, nil, true
			}
			dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
			count, err := taskOpsApplyBulk(v, items, "Set Due Date",
				func(project *models.Project, taskID string) bool {
					project.MutateTask(taskID, func(t *models.Task) {
						d := dueDate
						t.DueDate = &d
						t.HasTime = false
					})
					return true
				})
			if err != nil {
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			v.setStatus(fmt.Sprintf("Due date %s on %d tasks", dueDateLabel(days), count))
			return v, nil, true
		}

	case "m":
		if section, ok := sectionForMoveKey(key); ok {
			count, err := taskOpsApplyBulk(v, items, "Move to Section",
				func(project *models.Project, taskID string) bool {
					task := project.GetTaskByID(taskID)
					if task == nil || task.InSection(section) {
						return false
					}
					project.MutateTask(taskID, func(t *models.Task) {
						t.SetSection(section)
					})
					return true
				})
			if err != nil {
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			if count == 0 {
				v.setStatus("No tasks were moved")
			} else {
				v.setStatus(fmt.Sprintf("Moved %d tasks to %s", count, section))
			}
			return v, nil, true
		}

	case "s":
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if ok {
			count, err := taskOpsApplyBulk(v, items, "Snooze",
				func(project *models.Project, taskID string) bool {
					project.MutateTask(taskID, func(t *models.Task) {
						t.SnoozedUntil = snoozeDate
					})
					return true
				})
			if err != nil {
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			v.setStatus(fmt.Sprintf("Snoozed %d tasks", count))
			return v, nil, true
		}

	case "alt+.":
		if stage := StageForPrefixKey(key, v.getConfig().Stages); stage != nil {
			count, err := taskOpsApplyBulk(v, items, "Set Stage",
				func(project *models.Project, taskID string) bool {
					task := project.GetTaskByID(taskID)
					if task == nil || task.Stage == *stage {
						return false
					}
					project.MutateTask(taskID, func(t *models.Task) {
						t.Stage = *stage
					})
					return true
				})
			if err != nil {
				v.setStatus(fmt.Sprintf("Error: %v", err))
				return v, nil, true
			}
			if count == 0 {
				v.setStatus("No tasks were changed")
			} else {
				v.setStatus(fmt.Sprintf("Set stage on %d tasks", count))
			}
			return v, nil, true
		}
	}

	return nil, nil, false
}
