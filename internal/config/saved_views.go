package config

import "strings"

// GetSavedViews returns the list of saved views.
func (c *Config) GetSavedViews() []SavedView {
	if c.SavedViews == nil {
		return []SavedView{}
	}
	return c.SavedViews
}

// GetSavedView returns a saved view by name (case-insensitive).
// Returns nil if not found.
func (c *Config) GetSavedView(name string) *SavedView {
	nameLower := strings.ToLower(name)
	for i := range c.SavedViews {
		if strings.ToLower(c.SavedViews[i].Name) == nameLower {
			return &c.SavedViews[i]
		}
	}
	return nil
}

// SaveView upserts a saved view by case-insensitive name.
// If a view with the same name already exists, it is replaced.
func (c *Config) SaveView(view SavedView) {
	nameLower := strings.ToLower(view.Name)
	for i := range c.SavedViews {
		if strings.ToLower(c.SavedViews[i].Name) == nameLower {
			c.SavedViews[i] = view
			return
		}
	}
	c.SavedViews = append(c.SavedViews, view)
}

// DeleteSavedView removes a saved view by name (case-insensitive).
// Returns true if the view existed and was deleted.
func (c *Config) DeleteSavedView(name string) bool {
	nameLower := strings.ToLower(name)
	for i := range c.SavedViews {
		if strings.ToLower(c.SavedViews[i].Name) == nameLower {
			c.SavedViews = append(c.SavedViews[:i], c.SavedViews[i+1:]...)
			return true
		}
	}
	return false
}
