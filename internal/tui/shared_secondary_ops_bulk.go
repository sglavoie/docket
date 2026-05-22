package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
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
