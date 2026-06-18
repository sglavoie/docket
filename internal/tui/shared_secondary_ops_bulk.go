package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TaskProjectRef represents a task and its owning project for bulk operations.
type TaskProjectRef struct {
	Task    *models.Task
	Project *models.Project
}

// TaskProjectGroup groups task refs belonging to a single project.
type TaskProjectGroup struct {
	Project *models.Project
	Items   []TaskProjectRef
}

type changedTaskProjectGroup struct {
	group      TaskProjectGroup
	taskIDs    []string
	prevStates map[string]*models.Task
}

// bulkTaskItem is kept as a compatibility alias for existing view helpers.
type bulkTaskItem = TaskProjectRef

// bulkProjectGroup is kept as a compatibility alias for existing view helpers.
type bulkProjectGroup = TaskProjectGroup

// groupTaskProjectRefsByProject groups task refs by project, preserving the
// first-seen project order and filtering refs that cannot be applied.
func groupTaskProjectRefsByProject(refs []TaskProjectRef) []TaskProjectGroup {
	seen := make(map[string]int) // projectID -> index in result
	var result []TaskProjectGroup
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil {
			continue
		}
		idx, ok := seen[ref.Project.ID]
		if !ok {
			idx = len(result)
			seen[ref.Project.ID] = idx
			result = append(result, TaskProjectGroup{Project: ref.Project})
		}
		result[idx].Items = append(result[idx].Items, ref)
	}
	return result
}

// groupBulkItemsByProject groups items by project, preserving insertion order.
func groupBulkItemsByProject(items []bulkTaskItem) []bulkProjectGroup {
	return groupTaskProjectRefsByProject(items)
}

// taskIDsFromTaskRefs extracts task IDs in input order, skipping invalid refs.
func taskIDsFromTaskRefs(refs []TaskProjectRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil || ref.Task.ID == "" {
			continue
		}
		ids = append(ids, ref.Task.ID)
	}
	return ids
}

// taskIDsByProjectFromTaskRefs groups selected task IDs by owning project.
// Project order is available from groupTaskProjectRefsByProject; map values
// preserve first-seen task order within each project.
func taskIDsByProjectFromTaskRefs(refs []TaskProjectRef) map[string][]string {
	groups := groupTaskProjectRefsByProject(refs)
	idsByProject := make(map[string][]string, len(groups))
	for _, group := range groups {
		idsByProject[group.Project.ID] = taskIDsFromTaskRefs(group.Items)
	}
	return idsByProject
}

// affectedProjectsFromTaskRefs returns each affected project once in first-seen order.
func affectedProjectsFromTaskRefs(refs []TaskProjectRef) []*models.Project {
	groups := groupTaskProjectRefsByProject(refs)
	projects := make([]*models.Project, 0, len(groups))
	for _, group := range groups {
		projects = append(projects, group.Project)
	}
	return projects
}

// saveAffectedProjectsAsync saves all projects touched by refs with the common
// loading UI used by bulk AppModel handlers.
func saveAffectedProjectsAsync(refs []TaskProjectRef, successMsg string) tea.Cmd {
	projects := affectedProjectsFromTaskRefs(refs)
	if len(projects) == 0 {
		return nil
	}
	return tea.Batch(
		startLoading("Saving..."),
		SaveMultipleProjectsAsync(projects, successMsg),
	)
}

func workflowGroupMetadata(groups []TaskProjectGroup) (string, []string) {
	seen := make(map[string]bool, len(groups))
	projectIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.Project == nil || group.Project.ID == "" || seen[group.Project.ID] {
			continue
		}
		seen[group.Project.ID] = true
		projectIDs = append(projectIDs, group.Project.ID)
	}
	if len(projectIDs) <= 1 {
		return "", nil
	}
	sort.Strings(projectIDs)
	return uuid.New().String(), projectIDs
}

func taskProjectGroupsFromChanged(changed []changedTaskProjectGroup) []TaskProjectGroup {
	groups := make([]TaskProjectGroup, 0, len(changed))
	for _, item := range changed {
		groups = append(groups, item.group)
	}
	return groups
}

func sortedTaskIDsFromSet(ids map[string]bool) []string {
	taskIDs := make([]string, 0, len(ids))
	for id := range ids {
		if id != "" {
			taskIDs = append(taskIDs, id)
		}
	}
	sort.Strings(taskIDs)
	return taskIDs
}

// finishTaskBulkSelection clears task multi-select state after a successful bulk action.
func finishTaskBulkSelection(state *MultiSelectState, statusMsg *string, msg string) {
	if state == nil || statusMsg == nil {
		return
	}
	state.FinishBulk(statusMsg, msg)
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
	var changedGroups []changedTaskProjectGroup

	for _, group := range byProject {
		project := group.Project
		var changedIDs []string

		for _, item := range group.Items {
			if mutate(project, item.Task.ID) {
				total++
				changedIDs = append(changedIDs, item.Task.ID)
			}
		}
		if len(changedIDs) > 0 {
			changedGroups = append(changedGroups, changedTaskProjectGroup{group: group, taskIDs: changedIDs})
		}
	}

	groupID, groupProjectIDs := workflowGroupMetadata(taskProjectGroupsFromChanged(changedGroups))
	for _, changed := range changedGroups {
		undoCtx := fmt.Sprintf("%d tasks", len(changed.taskIDs))
		opts := BuildWorkflowOpts(
			v.getPersister(), changed.group.Project, v.getConfig(), v.getConfigManager(),
			changed.prevStates, undoAction, undoCtx, changed.taskIDs,
		)
		opts.UndoGroupID = groupID
		opts.UndoGroupProjectIDs = groupProjectIDs
		if err := persistWithWorkflow(opts); err != nil {
			return total, err
		}
	}

	v.reloadData()
	v.clampCursor()
	return total, nil
}
