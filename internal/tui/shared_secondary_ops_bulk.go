package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TaskProjectRef represents a task and its owning project across single-task and
// bulk TUI flows.
type TaskProjectRef struct {
	Task    *models.Task
	Project *models.Project
}

// TaskProjectGroup groups task refs belonging to a single project.
type TaskProjectGroup struct {
	Project *models.Project
	Items   []TaskProjectRef
}

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
func taskLocatorsFromTaskRefs(refs []TaskProjectRef) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(refs))
	for _, ref := range refs {
		locator := ref.Locator()
		if !locator.Valid() {
			continue
		}
		locators = append(locators, locator)
	}
	return locators
}

// taskIDsByProjectFromTaskRefs groups selected task IDs by owning project.
// Project order is available from groupTaskProjectRefsByProject; map values
// preserve first-seen task order within each project.
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
