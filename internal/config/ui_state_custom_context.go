package config

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// GetCustomContextNames returns a sorted list of custom context names.
func (c *Config) GetCustomContextNames() []string {
	if c.CustomContexts == nil {
		return []string{}
	}
	names := make([]string, 0, len(c.CustomContexts))
	for name := range c.CustomContexts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// GetCustomContext returns the project IDs for a custom context.
// Returns nil if the custom context doesn't exist.
// Note: this returns nil for missing contexts AND nil-valued contexts; callers
// that need to distinguish "exists with no projects" from "does not exist"
// should use CustomContextExists.
func (c *Config) GetCustomContext(name string) []string {
	key, ok := c.findCustomContextKey(name)
	if !ok {
		return nil
	}
	return c.CustomContexts[key]
}

// SetCustomContext sets the project IDs for a custom context.
// If the context already exists (case-insensitive), it updates the existing entry.
func (c *Config) SetCustomContext(name string, projectIDs []string) {
	if c.CustomContexts == nil {
		c.CustomContexts = make(map[string][]string)
	}
	if key, ok := c.findCustomContextKey(name); ok {
		delete(c.CustomContexts, key)
	}
	c.CustomContexts[name] = projectIDs
}

// DeleteCustomContext removes a custom context by name (case-insensitive).
// Returns true if the context existed and was deleted.
func (c *Config) DeleteCustomContext(name string) bool {
	key, ok := c.findCustomContextKey(name)
	if !ok {
		return false
	}

	delete(c.CustomContexts, key)
	if strings.EqualFold(c.FilterCustomContext, name) {
		c.FilterCustomContext = ""
	}
	return true
}

// CustomContextExists checks if a custom context exists (case-insensitive).
// A context with an empty or nil project list still counts as existing — only
// the presence of the map key matters. Previously this relied on
// GetCustomContext != nil, which falsely reported empty/nil-valued contexts
// as missing.
func (c *Config) CustomContextExists(name string) bool {
	_, ok := c.findCustomContextKey(name)
	return ok
}

// GetCustomContextProjectCount returns the count of active projects in a custom context.
func (c *Config) GetCustomContextProjectCount(name string) int {
	return len(c.GetCustomContextProjects(name))
}

// GetCustomContextArchivedProjectCount returns how many stored custom-context
// project IDs are hidden because the referenced projects are archived.
func (c *Config) GetCustomContextArchivedProjectCount(name string) int {
	return c.archivedProjectCountForIDs(c.GetCustomContext(name))
}

// GetCustomContextProjects returns the active projects in a custom context.
func (c *Config) GetCustomContextProjects(name string) []*models.Project {
	return c.activeProjectsForIDs(c.GetCustomContext(name))
}

// IsProjectInCustomContext checks if a project is in a custom context.
func (c *Config) IsProjectInCustomContext(customContextName, projectID string) bool {
	return slices.Contains(c.GetCustomContext(customContextName), projectID)
}

func (c *Config) activeProjectsForIDs(ids []string) []*models.Project {
	if ids == nil {
		return nil
	}

	var projects []*models.Project
	for _, id := range ids {
		if p := c.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

func (c *Config) archivedProjectCountForIDs(ids []string) int {
	if ids == nil {
		return 0
	}

	count := 0
	for _, id := range ids {
		if p := c.GetProject(id); p != nil && p.Archived {
			count++
		}
	}
	return count
}

func (c *Config) findCustomContextKey(name string) (string, bool) {
	if c.CustomContexts == nil {
		return "", false
	}

	for key := range c.CustomContexts {
		if strings.EqualFold(key, name) {
			return key, true
		}
	}
	return "", false
}
