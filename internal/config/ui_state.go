package config

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// RecentPaletteItem tracks a recently selected item in the command palette.
// Used to persist recent selections across sessions for boosted search results.
type RecentPaletteItem struct {
	Category string `json:"category"`        // ItemCategory as string (Commands, Tasks, Projects, Tags, Sections)
	ID       string `json:"id"`              // Command name, task ID, project slug, tag, or section name
	Count    int    `json:"count,omitempty"` // How many times this item has been selected
}

// GetDefaultProjectView returns the configured default project view.
// Returns "outliner" if not set or if the value is invalid.
func (c *Config) GetDefaultProjectView() string {
	switch c.DefaultProjectView {
	case "outliner", "project_tree":
		return c.DefaultProjectView
	default:
		return "outliner"
	}
}

// GetStartView returns the configured start view string.
// Returns "dashboard" for invalid or missing values.
func (c *Config) GetStartView() string {
	switch c.StartView {
	case "focus-board", "cross-project-outliner", "all-tasks",
		"daily-review", "backlog", "priority", "due-dates":
		return c.StartView
	default:
		return "dashboard"
	}
}

// GetTaskFormCollapsed returns whether the task form starts collapsed.
// Defaults to false (expanded) when not set to preserve existing behavior.
func (c *Config) GetTaskFormCollapsed() bool {
	if c.TaskFormCollapsed == nil {
		return false
	}
	return *c.TaskFormCollapsed
}

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

// InfoPanelMode represents the dashboard info panel display mode.
type InfoPanelMode int

const (
	InfoPanelOff            InfoPanelMode = 0
	InfoPanelInfoCards      InfoPanelMode = 1
	InfoPanelProjectSummary InfoPanelMode = 2
	infoPanelModeCount                    = 3
)

// GetInfoPanelMode returns the current info panel mode.
// If InfoPanelMode is nil, migrates from legacy ShowInfoPanel bool.
func (c *Config) GetInfoPanelMode() InfoPanelMode {
	if c.InfoPanelMode == nil {
		// Migrate from legacy ShowInfoPanel
		if c.ShowInfoPanel == nil || *c.ShowInfoPanel {
			return InfoPanelInfoCards
		}
		return InfoPanelOff
	}
	mode := InfoPanelMode(*c.InfoPanelMode)
	if mode < 0 || mode >= infoPanelModeCount {
		return InfoPanelInfoCards
	}
	return mode
}

// SetInfoPanelMode sets the info panel mode and clears the legacy ShowInfoPanel field.
func (c *Config) SetInfoPanelMode(mode InfoPanelMode) {
	m := int(mode)
	c.InfoPanelMode = &m
	c.ShowInfoPanel = nil
}

// CycleInfoPanelMode advances to the next info panel mode and returns it.
func (c *Config) CycleInfoPanelMode() InfoPanelMode {
	next := (c.GetInfoPanelMode() + 1) % infoPanelModeCount
	c.SetInfoPanelMode(next)
	return next
}

// GetShowInfoPanel returns whether to show the info panel section on dashboard.
// Delegates to GetInfoPanelMode for backward compatibility.
func (c *Config) GetShowInfoPanel() bool {
	return c.GetInfoPanelMode() != InfoPanelOff
}

// SetShowInfoPanel sets whether to show the info panel section on dashboard.
func (c *Config) SetShowInfoPanel(show bool) {
	c.ShowInfoPanel = &show
}

// GetTutorDismissed returns whether the tutor prompt has been dismissed.
// Defaults to false if not set.
func (c *Config) GetTutorDismissed() bool {
	if c.TutorDismissed == nil {
		return false
	}
	return *c.TutorDismissed
}

// SetTutorDismissed sets whether the tutor prompt has been dismissed.
func (c *Config) SetTutorDismissed(dismissed bool) {
	c.TutorDismissed = &dismissed
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
	return slices.Contains(c.PinnedCommands, name)
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
func (c *Config) GetCustomContext(name string) []string {
	if c.CustomContexts == nil {
		return nil
	}
	// Case-insensitive lookup
	nameLower := strings.ToLower(name)
	for k, v := range c.CustomContexts {
		if strings.ToLower(k) == nameLower {
			return v
		}
	}
	return nil
}

// SetCustomContext sets the project IDs for a custom context.
// If the context already exists (case-insensitive), it updates the existing entry.
func (c *Config) SetCustomContext(name string, projectIDs []string) {
	if c.CustomContexts == nil {
		c.CustomContexts = make(map[string][]string)
	}
	// Case-insensitive check for existing key
	nameLower := strings.ToLower(name)
	for k := range c.CustomContexts {
		if strings.ToLower(k) == nameLower {
			delete(c.CustomContexts, k)
			break
		}
	}
	c.CustomContexts[name] = projectIDs
}

// DeleteCustomContext removes a custom context by name (case-insensitive).
// Returns true if the context existed and was deleted.
func (c *Config) DeleteCustomContext(name string) bool {
	if c.CustomContexts == nil {
		return false
	}
	nameLower := strings.ToLower(name)
	for k := range c.CustomContexts {
		if strings.ToLower(k) == nameLower {
			delete(c.CustomContexts, k)
			// Clear filter if this was the active custom context
			if strings.ToLower(c.FilterCustomContext) == nameLower {
				c.FilterCustomContext = ""
			}
			return true
		}
	}
	return false
}

// CustomContextExists checks if a custom context exists (case-insensitive).
func (c *Config) CustomContextExists(name string) bool {
	return c.GetCustomContext(name) != nil
}

// GetCustomContextProjectCount returns the count of active projects in a custom context.
func (c *Config) GetCustomContextProjectCount(name string) int {
	projectIDs := c.GetCustomContext(name)
	if projectIDs == nil {
		return 0
	}
	count := 0
	for _, id := range projectIDs {
		if p := c.GetProject(id); p != nil && !p.Archived {
			count++
		}
	}
	return count
}

// GetCustomContextProjects returns the active projects in a custom context.
func (c *Config) GetCustomContextProjects(name string) []*models.Project {
	projectIDs := c.GetCustomContext(name)
	if projectIDs == nil {
		return nil
	}
	var projects []*models.Project
	for _, id := range projectIDs {
		if p := c.GetProject(id); p != nil && !p.Archived {
			projects = append(projects, p)
		}
	}
	return projects
}

// IsProjectInCustomContext checks if a project is in a custom context.
func (c *Config) IsProjectInCustomContext(customContextName, projectID string) bool {
	projectIDs := c.GetCustomContext(customContextName)
	return slices.Contains(projectIDs, projectID)
}
