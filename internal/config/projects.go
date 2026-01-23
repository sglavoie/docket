package config

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// GetProject returns a project by ID or slug.
func (c *Config) GetProject(idOrSlug string) *models.Project {
	for _, p := range c.Projects {
		if p.ID == idOrSlug || p.Slug == idOrSlug || p.Name == idOrSlug {
			return p
		}
	}
	return nil
}

// GetCurrentProject returns the currently selected project.
func (c *Config) GetCurrentProject() *models.Project {
	if c.CurrentProject == "" {
		return nil
	}
	return c.GetProject(c.CurrentProject)
}

// AddProject adds a project to the config.
func (c *Config) AddProject(project *models.Project) {
	c.Projects = append(c.Projects, project)
}

// RemoveProject removes a project by ID.
func (c *Config) RemoveProject(id string) bool {
	for i, p := range c.Projects {
		if p.ID == id {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			if c.CurrentProject == id {
				c.CurrentProject = ""
			}
			return true
		}
	}
	return false
}

// sortProjectsByName sorts projects alphabetically by name (case-insensitive).
func sortProjectsByName(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// sortProjectsWithPinnedFirst sorts projects with pinned ones first, then non-pinned.
// Each group is sorted alphabetically by name (case-insensitive).
func sortProjectsWithPinnedFirst(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Pinned != projects[j].Pinned {
			return projects[i].Pinned
		}
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

// GetActiveProjects returns non-archived projects sorted with pinned first, then alphabetically.
func (c *Config) GetActiveProjects() []*models.Project {
	var active []*models.Project
	for _, p := range c.Projects {
		if !p.Archived {
			active = append(active, p)
		}
	}
	sortProjectsWithPinnedFirst(active)
	return active
}

// GetArchivedProjects returns archived projects sorted alphabetically.
func (c *Config) GetArchivedProjects() []*models.Project {
	var archived []*models.Project
	for _, p := range c.Projects {
		if p.Archived {
			archived = append(archived, p)
		}
	}
	sortProjectsByName(archived)
	return archived
}

// SetCurrentProject sets the current project by ID.
func (c *Config) SetCurrentProject(id string) bool {
	project := c.GetProject(id)
	if project == nil {
		return false
	}
	c.CurrentProject = project.ID
	return true
}

// GetAllContexts returns a sorted list of unique context values from all projects.
func (c *Config) GetAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range c.Projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
}
