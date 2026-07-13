package tui

import (
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

// bulkTaskItem is kept as a compatibility alias for existing view helpers.
type bulkTaskItem = TaskProjectRef

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

// finishTaskBulkSelection clears task multi-select state after a successful bulk action.
func finishTaskBulkSelection(state *MultiSelectState, statusMsg *string, msg string) {
	if state == nil || statusMsg == nil {
		return
	}
	state.FinishBulk(statusMsg, msg)
}
