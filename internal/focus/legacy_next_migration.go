package focus

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// LegacyNextOrderMemberLocators returns valid project-qualified identities from
// a legacy ordering-only index. Schema-6 migration converts those identities to
// explicit Next membership; missing and terminal entries are omitted.
//
// The returned locators preserve queue order and are de-duplicated. This
// function is deliberately side-effect free: it names what to migrate but never
// mutates task state or the order index. A queue with no legacy members yields
// an empty slice.
func LegacyNextOrderMemberLocators(order []config.ProjectTaskLocator, projects []*models.Project) []config.ProjectTaskLocator {
	projectsByID := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			projectsByID[project.ID] = project
		}
	}
	seen := make(map[config.ProjectTaskLocator]bool)
	var locators []config.ProjectTaskLocator
	for _, locator := range order {
		project := projectsByID[locator.ProjectID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(locator.TaskID)
		if task != nil && !task.IsTerminalState() && !seen[locator] {
			seen[locator] = true
			locators = append(locators, locator)
		}
	}
	return locators
}
