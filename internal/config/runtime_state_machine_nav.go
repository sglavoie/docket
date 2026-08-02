package config

import "github.com/sglavoie/tt/internal/models"

// GetProject rehydrates a cataloged project by ID or slug, or nil if none
// matches. It mirrors (*Config).GetProject over the in-memory catalog snapshot.
func (rs *RuntimeState) GetProject(idOrSlug string) *models.Project {
	if rs == nil || idOrSlug == "" {
		return nil
	}
	for _, p := range rs.catalogProjects() {
		if p.ID == idOrSlug || p.Slug == idOrSlug {
			return p
		}
	}
	return nil
}
