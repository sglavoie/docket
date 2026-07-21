package config

import "github.com/sglavoie/tt/internal/models"

// This file holds the catalog-combining navigation accessors on RuntimeState:
// the machine-local domain owns the cycle/context project ID lists and the
// current-project selection, but rehydrating those IDs into *models.Project
// requires the workspace catalog. Each mirrors its (*Config) counterpart but
// reads the in-memory Machine + Catalog domains instead of the facade, so the
// project-cycle manager, context pickers, and navigation helpers stay disk-free
// and off config.Config. The archived filter matches the facade: a referenced
// project that no longer exists or is archived is dropped from the returned
// list (but still counted by the *ArchivedCount* helpers).

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

// CurrentProject rehydrates the machine-local current-project selection, or nil
// when none is selected. It mirrors (*Config).GetCurrentProject.
func (rs *RuntimeState) CurrentProject() *models.Project {
	if rs == nil {
		return nil
	}
	id := rs.Machine().CurrentProject
	if id == "" {
		return nil
	}
	return rs.GetProject(id)
}

// CustomContextProjects mirrors (*Config).GetCustomContextProjects.
func (rs *RuntimeState) CustomContextProjects(name string) []*models.Project {
	return rs.activeProjectsForIDs(rs.Machine().GetCustomContext(name))
}

// CustomContextProjectCount mirrors (*Config).GetCustomContextProjectCount.
func (rs *RuntimeState) CustomContextProjectCount(name string) int {
	return len(rs.CustomContextProjects(name))
}

// CustomContextArchivedProjectCount mirrors
// (*Config).GetCustomContextArchivedProjectCount.
func (rs *RuntimeState) CustomContextArchivedProjectCount(name string) int {
	return rs.archivedProjectCountForIDs(rs.Machine().GetCustomContext(name))
}

// activeProjectsForIDs mirrors (*Config).activeProjectsForIDs: nil ids yield a
// nil slice (distinct from an empty non-nil result for a known-but-empty set).
func (rs *RuntimeState) activeProjectsForIDs(ids []string) []*models.Project {
	if ids == nil {
		return nil
	}
	var projects []*models.Project
	for _, id := range ids {
		if p := rs.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

// archivedProjectCountForIDs mirrors (*Config).archivedProjectCountForIDs.
func (rs *RuntimeState) archivedProjectCountForIDs(ids []string) int {
	if ids == nil {
		return 0
	}
	count := 0
	for _, id := range ids {
		if p := rs.GetProject(id); p != nil && p.Archived {
			count++
		}
	}
	return count
}
