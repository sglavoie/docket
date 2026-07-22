package focus

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// LegacyNextQueueMemberLocators returns the project-qualified identities of
// Focus Queue members that predate the state-driven Next invariant and must be
// backfilled to StateNext.
// Next membership is now the StateNext workflow state, so a queued task that
// exists and is non-terminal but is not yet StateNext is a legacy member whose
// only evidence lives in the ordering-only index; it must be migrated so
// first-use pruning cannot silently interpret it as stale and drop it. Missing
// and terminal members are left alone — those are genuinely stale and handled by
// the prune path.
//
// The returned locators preserve queue order and are de-duplicated. This
// function is deliberately side-effect free: it names what to migrate but never
// mutates task state or the order index. A queue with no legacy members yields
// an empty slice.
func LegacyNextQueueMemberLocators(queue []config.FocusQueueItem, projects []*models.Project) []config.ProjectTaskLocator {
	projectsByID := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			projectsByID[project.ID] = project
		}
	}
	seen := make(map[config.ProjectTaskLocator]bool)
	var locators []config.ProjectTaskLocator
	for _, item := range queue {
		project := projectsByID[item.ProjectID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(item.TaskID)
		locator := config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if task != nil && !task.IsTerminalState() && task.State() != models.StateNext && !seen[locator] {
			seen[locator] = true
			locators = append(locators, locator)
		}
	}
	return locators
}
