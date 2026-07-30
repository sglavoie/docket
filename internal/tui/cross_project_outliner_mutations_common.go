package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.loadedProjects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.loadedProjects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}
