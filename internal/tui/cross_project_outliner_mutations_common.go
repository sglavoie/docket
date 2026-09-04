package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.population.projects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.population.projects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}
