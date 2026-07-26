package config

import (
	"sort"

	"github.com/sglavoie/tt/internal/template"
)

// Task-template behavior on the AutomationConfig domain aggregate. These mirror
// the equivalent *Config facade methods but depend only on the automation
// domain, so template consumers can read and mutate templates through the narrow
// automation seam instead of the whole config.Config facade. GetEnabledTaskTemplates
// lives in automation_reads.go.

// GetTaskTemplates returns all task templates (never nil).
func (a *AutomationConfig) GetTaskTemplates() []template.TaskTemplate {
	if a.TaskTemplates == nil {
		return []template.TaskTemplate{}
	}
	return a.TaskTemplates
}

// GetTaskTemplatesSorted returns templates sorted default-first, then custom,
// each group ordered alphabetically by name.
func (a *AutomationConfig) GetTaskTemplatesSorted() (defaultTemplates, customTemplates []template.TaskTemplate) {
	for _, t := range a.TaskTemplates {
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
func (a *AutomationConfig) GetTaskTemplatesByCategory() map[string][]template.TaskTemplate {
	result := make(map[string][]template.TaskTemplate)
	for _, t := range a.TaskTemplates {
		if t.Enabled {
			cat := t.Category
			if cat == "" {
				cat = "other"
			}
			result[cat] = append(result[cat], t)
		}
	}
	for cat := range result {
		sort.Slice(result[cat], func(i, j int) bool {
			return result[cat][i].Name < result[cat][j].Name
		})
	}
	return result
}

// GetTaskTemplate returns a pointer to the template with the given ID, or nil.
// The pointer aliases the backing slice so in-place edits are observed.
func (a *AutomationConfig) GetTaskTemplate(id string) *template.TaskTemplate {
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].ID == id {
			return &a.TaskTemplates[i]
		}
	}
	return nil
}

// GetTaskTemplateByName returns a template by exact name match, or nil.
func (a *AutomationConfig) GetTaskTemplateByName(name string) *template.TaskTemplate {
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].Name == name {
			return &a.TaskTemplates[i]
		}
	}
	return nil
}

// HasTemplateWithID reports whether a template with the given ID exists.
func (a *AutomationConfig) HasTemplateWithID(id string) bool {
	for _, t := range a.TaskTemplates {
		if t.ID == id {
			return true
		}
	}
	return false
}

// AddTaskTemplate appends a new task template.
func (a *AutomationConfig) AddTaskTemplate(t template.TaskTemplate) {
	t.NormalizeState()
	a.TaskTemplates = append(a.TaskTemplates, t)
}

// UpdateTaskTemplate replaces an existing template by ID, returning true when found.
func (a *AutomationConfig) UpdateTaskTemplate(t template.TaskTemplate) bool {
	t.NormalizeState()
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].ID == t.ID {
			a.TaskTemplates[i] = t
			return true
		}
	}
	return false
}

// MigrateTaskTemplateStates makes every template's defaulted canonical state
// explicit and reports whether a legacy section or omitted state requires the
// automation artifact to be rewritten.
func (a *AutomationConfig) MigrateTaskTemplateStates() bool {
	if a == nil {
		return false
	}
	changed := false
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].NormalizeState() {
			changed = true
		}
	}
	return changed
}

// DeleteTaskTemplate removes a template by ID, returning true when found.
func (a *AutomationConfig) DeleteTaskTemplate(id string) bool {
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].ID == id {
			a.TaskTemplates = append(a.TaskTemplates[:i], a.TaskTemplates[i+1:]...)
			return true
		}
	}
	return false
}

// ToggleTaskTemplate flips a template's enabled state, returning the new state and
// whether the template was found.
func (a *AutomationConfig) ToggleTaskTemplate(id string) (enabled bool, found bool) {
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].ID == id {
			a.TaskTemplates[i].Enabled = !a.TaskTemplates[i].Enabled
			return a.TaskTemplates[i].Enabled, true
		}
	}
	return false, false
}

// IncrementTemplateUseCount increments a template's use count, returning true
// when found.
func (a *AutomationConfig) IncrementTemplateUseCount(id string) bool {
	for i := range a.TaskTemplates {
		if a.TaskTemplates[i].ID == id {
			a.TaskTemplates[i].UseCount++
			return true
		}
	}
	return false
}

// AddDefaultTemplates adds built-in default templates that do not already exist.
// When enableTemplates is true, added templates are created enabled. Returns the
// number of templates added.
func (a *AutomationConfig) AddDefaultTemplates(enableTemplates bool) int {
	defaults := template.GetDefaultTemplates()
	added := 0
	for _, t := range defaults {
		if !a.HasTemplateWithID(t.ID) {
			if enableTemplates {
				t.Enabled = true
			}
			a.TaskTemplates = append(a.TaskTemplates, t)
			added++
		}
	}
	a.DefaultTemplatesInit = true
	return added
}

// ResetChangedDefaultTemplates resets edited or deleted default templates
// (Source == "default") to their shipped values, mirroring
// Config.ResetChangedDefaultTemplates. Returns (updated, re-added, unchanged)
// counts and the affected template IDs.
func (a *AutomationConfig) ResetChangedDefaultTemplates(enableTemplates bool) (updated, readded, unchanged int, affectedIDs []string) {
	defaults := template.GetDefaultTemplates()
	defaultMap := make(map[string]template.TaskTemplate)
	for _, t := range defaults {
		defaultMap[t.ID] = t
	}

	// Phase 1: update existing default templates that have changed.
	for i := range a.TaskTemplates {
		existing := &a.TaskTemplates[i]
		if existing.Source != "default" {
			continue
		}
		shipped, exists := defaultMap[existing.ID]
		if !exists {
			continue
		}
		if !existing.ContentEqual(&shipped) {
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

	// Phase 2: re-add deleted default templates.
	for _, t := range defaultMap {
		t.Enabled = enableTemplates
		a.TaskTemplates = append(a.TaskTemplates, t)
		readded++
		affectedIDs = append(affectedIDs, t.ID)
	}

	return updated, readded, unchanged, affectedIDs
}
