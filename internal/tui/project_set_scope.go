package tui

import "github.com/sglavoie/tt/internal/models"

// projectsInProjectSet filters projects through an explicit typed scope while
// retaining the caller's stable project order. An omitted set preserves the
// legacy app-wide behavior; an explicit empty set intentionally returns none.
func projectsInProjectSet(projects []*models.Project, scope ProjectSetScope) []*models.Project {
	if !scope.Explicit {
		return append([]*models.Project(nil), projects...)
	}
	projectIDs := scope.ProjectIDs()
	admitted := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		admitted[projectID] = true
	}
	filtered := make([]*models.Project, 0, len(projectIDs))
	for _, project := range projects {
		if project != nil && admitted[project.ID] {
			filtered = append(filtered, project)
		}
	}
	return filtered
}
