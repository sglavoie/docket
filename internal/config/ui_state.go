package config

import "github.com/sglavoie/tt/internal/models"

// GetCycleProjects returns active projects that are in the cycle list, in order.
// Filters out archived projects and projects that no longer exist.
func (c *Config) GetCycleProjects() []*models.Project {
	var projects []*models.Project
	for _, id := range c.CycleProjects {
		if p := c.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

// SetCycleProjects sets the list of project IDs for quick cycling.
func (c *Config) SetCycleProjects(ids []string) {
	c.CycleProjects = ids
}

// GetCycleProjectIndex returns the index of a project in the cycle list, or -1 if not found.
func (c *Config) GetCycleProjectIndex(projectID string) int {
	cycleProjects := c.GetCycleProjects()
	for i, p := range cycleProjects {
		if p.ID == projectID {
			return i
		}
	}
	return -1
}

// GetCycleProjectsForContext returns active projects in the cycle list for a specific context.
// If context is empty, returns the global cycle projects.
func (c *Config) GetCycleProjectsForContext(context string) []*models.Project {
	if context == "" {
		return c.GetCycleProjects()
	}
	if c.ContextCycleProjects == nil {
		return nil
	}
	ids := c.ContextCycleProjects[context]
	var projects []*models.Project
	for _, id := range ids {
		if p := c.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

// GetCycleProjectIDsForContext returns project IDs in the cycle list for a specific context.
// If context is empty, returns the global cycle project IDs.
func (c *Config) GetCycleProjectIDsForContext(context string) []string {
	if context == "" {
		return c.CycleProjects
	}
	if c.ContextCycleProjects == nil {
		return nil
	}
	return c.ContextCycleProjects[context]
}

// SetCycleProjectsForContext sets the cycle project IDs for a specific context.
// If context is empty, sets the global cycle projects.
func (c *Config) SetCycleProjectsForContext(context string, ids []string) {
	if context == "" {
		c.CycleProjects = ids
		return
	}
	if c.ContextCycleProjects == nil {
		c.ContextCycleProjects = make(map[string][]string)
	}
	c.ContextCycleProjects[context] = ids
}

// GetShowRecentTasks returns whether to show the Recent Tasks section on dashboard.
// Defaults to true if not set.
func (c *Config) GetShowRecentTasks() bool {
	if c.ShowRecentTasks == nil {
		return true
	}
	return *c.ShowRecentTasks
}

// SetShowRecentTasks sets whether to show the Recent Tasks section on dashboard.
func (c *Config) SetShowRecentTasks(show bool) {
	c.ShowRecentTasks = &show
}

// GetRecentPaletteItems returns the list of recently selected items in the command palette.
func (c *Config) GetRecentPaletteItems() []RecentPaletteItem {
	if c.RecentPaletteItems == nil {
		return []RecentPaletteItem{}
	}
	return c.RecentPaletteItems
}

// SetRecentPaletteItems sets the list of recently selected items in the command palette.
func (c *Config) SetRecentPaletteItems(items []RecentPaletteItem) {
	c.RecentPaletteItems = items
}

// GetLastPaletteCategory returns the last category filter used in the command palette.
func (c *Config) GetLastPaletteCategory() string {
	return c.LastPaletteCategory
}

// SetLastPaletteCategory sets the last category filter used in the command palette.
func (c *Config) SetLastPaletteCategory(category string) {
	c.LastPaletteCategory = category
}

// GetPaletteSearchHistory returns the list of recent search queries in the command palette.
func (c *Config) GetPaletteSearchHistory() []string {
	if c.PaletteSearchHistory == nil {
		return []string{}
	}
	return c.PaletteSearchHistory
}

// SetPaletteSearchHistory sets the list of recent search queries in the command palette.
func (c *Config) SetPaletteSearchHistory(history []string) {
	c.PaletteSearchHistory = history
}

// GetPinnedCommands returns the list of pinned command names.
func (c *Config) GetPinnedCommands() []string {
	if c.PinnedCommands == nil {
		return []string{}
	}
	return c.PinnedCommands
}

// SetPinnedCommands sets the list of pinned command names.
func (c *Config) SetPinnedCommands(commands []string) {
	c.PinnedCommands = commands
}

// IsCommandPinned returns true if the command is in the pinned list.
func (c *Config) IsCommandPinned(name string) bool {
	for _, cmd := range c.PinnedCommands {
		if cmd == name {
			return true
		}
	}
	return false
}

// TogglePinnedCommand adds or removes a command from the pinned list.
// Returns true if the command is now pinned, false if unpinned.
func (c *Config) TogglePinnedCommand(name string) bool {
	for i, cmd := range c.PinnedCommands {
		if cmd == name {
			// Remove from pinned
			c.PinnedCommands = append(c.PinnedCommands[:i], c.PinnedCommands[i+1:]...)
			return false
		}
	}
	// Add to pinned
	c.PinnedCommands = append(c.PinnedCommands, name)
	return true
}

// GetSavedSearches returns the map of saved searches (name -> query).
func (c *Config) GetSavedSearches() map[string]string {
	if c.SavedSearches == nil {
		return make(map[string]string)
	}
	return c.SavedSearches
}

// SaveSearch saves a search query with a given name.
// Overwrites if name already exists.
func (c *Config) SaveSearch(name, query string) {
	if c.SavedSearches == nil {
		c.SavedSearches = make(map[string]string)
	}
	c.SavedSearches[name] = query
}

// DeleteSavedSearch removes a saved search by name.
// Returns true if the search existed and was deleted.
func (c *Config) DeleteSavedSearch(name string) bool {
	if c.SavedSearches == nil {
		return false
	}
	if _, exists := c.SavedSearches[name]; exists {
		delete(c.SavedSearches, name)
		return true
	}
	return false
}

// GetSavedSearch returns the query for a saved search by name.
// Returns empty string if not found.
func (c *Config) GetSavedSearch(name string) string {
	if c.SavedSearches == nil {
		return ""
	}
	return c.SavedSearches[name]
}

// GetCommandHistory returns the list of recently executed commands.
func (c *Config) GetCommandHistory() []string {
	if c.CommandHistory == nil {
		return []string{}
	}
	return c.CommandHistory
}

// AddCommandToHistory adds a command to the execution history.
// Most recent commands are at the front. Duplicates are moved to front.
// Capped at 10 entries.
func (c *Config) AddCommandToHistory(name string) {
	const maxHistory = 10

	// Remove existing entry if present
	var newHistory []string
	for _, cmd := range c.CommandHistory {
		if cmd != name {
			newHistory = append(newHistory, cmd)
		}
	}

	// Add to front
	c.CommandHistory = append([]string{name}, newHistory...)

	// Cap at max
	if len(c.CommandHistory) > maxHistory {
		c.CommandHistory = c.CommandHistory[:maxHistory]
	}
}
