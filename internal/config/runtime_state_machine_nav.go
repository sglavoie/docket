package config

import "github.com/sglavoie/tt/internal/models"

// This file holds the catalog-combining navigation accessors on RuntimeState:
// the machine-local domain owns custom-context project ID lists, but rehydrating
// those IDs into *models.Project requires the workspace catalog. Archived and
// missing references are excluded from active results while retaining their
// diagnostic counts.

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

// CustomContextStaleReferenceCount counts stored project IDs that no longer
// match any cataloged project. Unlike archived references, these cannot be
// rehydrated, so the stored ID list is left untouched and the count is reported
// separately.
func (rs *RuntimeState) CustomContextStaleReferenceCount(name string) int {
	return rs.missingProjectCountForIDs(rs.Machine().GetCustomContext(name))
}

// activeProjectsForIDs rehydrates the stored IDs that satisfy
// IsActiveContextMember, so a resolved context never contains a project the
// active-only cross-project loader will skip. nil ids yield a nil slice
// (distinct from an empty non-nil result for a known-but-empty set).
func (rs *RuntimeState) activeProjectsForIDs(ids []string) []*models.Project {
	if ids == nil {
		return nil
	}
	var projects []*models.Project
	for _, id := range ids {
		if p := rs.GetProject(id); IsActiveContextMember(p) {
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

// missingProjectCountForIDs counts IDs that resolve to no cataloged project.
func (rs *RuntimeState) missingProjectCountForIDs(ids []string) int {
	count := 0
	for _, id := range ids {
		if rs.GetProject(id) == nil {
			count++
		}
	}
	return count
}
