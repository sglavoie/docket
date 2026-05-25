package config

import (
	"sort"

	"github.com/sglavoie/tt/internal/template"
)

// GetTaskTemplates returns all task templates.
func (c *Config) GetTaskTemplates() []template.TaskTemplate {
	if c.TaskTemplates == nil {
		return []template.TaskTemplate{}
	}
	return c.TaskTemplates
}

// GetEnabledTaskTemplates returns only enabled task templates.
func (c *Config) GetEnabledTaskTemplates() []template.TaskTemplate {
	var enabled []template.TaskTemplate
	for _, t := range c.TaskTemplates {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// GetTaskTemplatesSorted returns templates sorted: default first, then custom.
// Within each group, templates are sorted alphabetically by name.
func (c *Config) GetTaskTemplatesSorted() (defaultTemplates, customTemplates []template.TaskTemplate) {
	for _, t := range c.TaskTemplates {
		if t.Source == "default" {
			defaultTemplates = append(defaultTemplates, t)
		} else {
			customTemplates = append(customTemplates, t)
		}
	}
	sort.Slice(defaultTemplates, func(i, j int) bool {
		return defaultTemplates[i].Name < defaultTemplates[j].Name
	})
	sort.Slice(customTemplates, func(i, j int) bool {
		return customTemplates[i].Name < customTemplates[j].Name
	})
	return defaultTemplates, customTemplates
}

// GetTaskTemplatesByCategory returns enabled templates grouped by category.
func (c *Config) GetTaskTemplatesByCategory() map[string][]template.TaskTemplate {
	result := make(map[string][]template.TaskTemplate)
	for _, t := range c.TaskTemplates {
		if t.Enabled {
			cat := t.Category
			if cat == "" {
				cat = "other"
			}
			result[cat] = append(result[cat], t)
		}
	}
	// Sort each category alphabetically
	for cat := range result {
		sort.Slice(result[cat], func(i, j int) bool {
			return result[cat][i].Name < result[cat][j].Name
		})
	}
	return result
}

// GetTaskTemplate returns a task template by ID.
func (c *Config) GetTaskTemplate(id string) *template.TaskTemplate {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].ID == id {
			return &c.TaskTemplates[i]
		}
	}
	return nil
}

// GetTaskTemplateByName returns a task template by exact name match.
func (c *Config) GetTaskTemplateByName(name string) *template.TaskTemplate {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].Name == name {
			return &c.TaskTemplates[i]
		}
	}
	return nil
}

// AddTaskTemplate adds a new task template.
func (c *Config) AddTaskTemplate(t template.TaskTemplate) {
	c.TaskTemplates = append(c.TaskTemplates, t)
}

// UpdateTaskTemplate updates an existing task template by ID.
// Returns true if the template was found and updated.
func (c *Config) UpdateTaskTemplate(t template.TaskTemplate) bool {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].ID == t.ID {
			c.TaskTemplates[i] = t
			return true
		}
	}
	return false
}

// DeleteTaskTemplate removes a task template by ID.
// Returns true if the template was found and deleted.
func (c *Config) DeleteTaskTemplate(id string) bool {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].ID == id {
			c.TaskTemplates = append(c.TaskTemplates[:i], c.TaskTemplates[i+1:]...)
			return true
		}
	}
	return false
}

// ToggleTaskTemplate enables or disables a task template by ID.
// Returns the new enabled state, or false if the template was not found.
func (c *Config) ToggleTaskTemplate(id string) (enabled bool, found bool) {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].ID == id {
			c.TaskTemplates[i].Enabled = !c.TaskTemplates[i].Enabled
			return c.TaskTemplates[i].Enabled, true
		}
	}
	return false, false
}

// HasTemplateWithID returns true if a template with the given ID exists.
func (c *Config) HasTemplateWithID(id string) bool {
	for _, t := range c.TaskTemplates {
		if t.ID == id {
			return true
		}
	}
	return false
}

// IncrementTemplateUseCount increments the use count for a template by ID.
// Returns true if the template was found.
func (c *Config) IncrementTemplateUseCount(id string) bool {
	for i := range c.TaskTemplates {
		if c.TaskTemplates[i].ID == id {
			c.TaskTemplates[i].UseCount++
			return true
		}
	}
	return false
}

// AddDefaultTemplates adds the built-in default templates that don't already exist.
// If enableTemplates is true, templates are created enabled.
// Returns the number of templates added.
func (c *Config) AddDefaultTemplates(enableTemplates bool) int {
	defaults := template.GetDefaultTemplates()
	added := 0
	for _, t := range defaults {
		if !c.HasTemplateWithID(t.ID) {
			if enableTemplates {
				t.Enabled = true
			}
			c.TaskTemplates = append(c.TaskTemplates, t)
			added++
		}
	}
	c.DefaultTemplatesInit = true
	return added
}

// ResetChangedDefaultTemplates resets any default templates (Source == "default")
// that have been edited or deleted to their shipped values.
// Returns counts: (updated, re-added, unchanged) and a slice of affected template IDs.
func (c *Config) ResetChangedDefaultTemplates(enableTemplates bool) (updated, readded, unchanged int, affectedIDs []string) {
	defaults := template.GetDefaultTemplates()
	defaultMap := make(map[string]template.TaskTemplate)

	// Build map of shipped defaults by ID
	for _, t := range defaults {
		defaultMap[t.ID] = t
	}

	// Phase 1: Update existing default templates that have changed
	for i := range c.TaskTemplates {
		existing := &c.TaskTemplates[i]
		if existing.Source != "default" {
			continue // Skip custom templates
		}

		shipped, exists := defaultMap[existing.ID]
		if !exists {
			continue
		}

		// Compare template content (semantic only, ignore metadata).
		// ContentEqual treats Tags/Checklist/Children as ordered lists so
		// tag-order edits are correctly detected as differences.
		if !existing.ContentEqual(&shipped) {
			// Reset to shipped values, preserve Enabled state unless enableTemplates is true
			preservedEnabled := existing.Enabled
			preservedUseCount := existing.UseCount
			*existing = shipped
			if enableTemplates {
				existing.Enabled = true
			} else {
				existing.Enabled = preservedEnabled
			}
			existing.UseCount = preservedUseCount
			updated++
			affectedIDs = append(affectedIDs, existing.ID)
		} else {
			unchanged++
		}

		delete(defaultMap, existing.ID)
	}

	// Phase 2: Re-add deleted default templates
	for _, t := range defaultMap {
		if enableTemplates {
			t.Enabled = true
		} else {
			t.Enabled = false
		}
		c.TaskTemplates = append(c.TaskTemplates, t)
		readded++
		affectedIDs = append(affectedIDs, t.ID)
	}

	return updated, readded, unchanged, affectedIDs
}
