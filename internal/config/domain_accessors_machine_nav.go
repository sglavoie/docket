package config

import (
	"slices"
	"strings"
)

// This file mirrors the config.Config facade's machine-local navigation,
// context, history, and recent-selection accessors onto the MachineState
// domain, so navigation/context/history/recent consumers can read and mutate
// machine-local state directly through RuntimeState.Machine() with identical
// defaulting/normalization behavior. The bodies mirror their (*Config)
// counterparts in ui_state.go, ui_state_custom_context.go, and recent_visits.go
// field-for-field. Catalog-combining variants (returning *models.Project) live
// on RuntimeState in runtime_state_machine_nav.go, because they need the
// workspace catalog to rehydrate projects.

// --- MachineState: cycle project IDs ---------------------------------------

// GetCycleProjectIDsForContext mirrors (*Config).GetCycleProjectIDsForContext.
func (m *MachineState) GetCycleProjectIDsForContext(context string) []string {
	if context == "" {
		return m.CycleProjects
	}
	if m.ContextCycleProjects == nil {
		return nil
	}
	return m.ContextCycleProjects[context]
}

// SetCycleProjects mirrors (*Config).SetCycleProjects.
func (m *MachineState) SetCycleProjects(ids []string) {
	m.CycleProjects = ids
}

// SetCycleProjectsForContext mirrors (*Config).SetCycleProjectsForContext.
func (m *MachineState) SetCycleProjectsForContext(context string, ids []string) {
	if context == "" {
		m.CycleProjects = ids
		return
	}
	if len(ids) == 0 {
		if m.ContextCycleProjects != nil {
			delete(m.ContextCycleProjects, context)
		}
		return
	}
	if m.ContextCycleProjects == nil {
		m.ContextCycleProjects = make(map[string][]string)
	}
	m.ContextCycleProjects[context] = ids
}

// --- MachineState: custom contexts -----------------------------------------

func (m *MachineState) findCustomContextKey(name string) (string, bool) {
	if m.CustomContexts == nil {
		return "", false
	}
	for key := range m.CustomContexts {
		if strings.EqualFold(key, name) {
			return key, true
		}
	}
	return "", false
}

// GetCustomContextNames mirrors (*Config).GetCustomContextNames.
func (m *MachineState) GetCustomContextNames() []string {
	if m.CustomContexts == nil {
		return []string{}
	}
	names := make([]string, 0, len(m.CustomContexts))
	for name := range m.CustomContexts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// GetCustomContext mirrors (*Config).GetCustomContext.
func (m *MachineState) GetCustomContext(name string) []string {
	key, ok := m.findCustomContextKey(name)
	if !ok {
		return nil
	}
	return m.CustomContexts[key]
}

// SetCustomContext mirrors (*Config).SetCustomContext.
func (m *MachineState) SetCustomContext(name string, projectIDs []string) {
	if m.CustomContexts == nil {
		m.CustomContexts = make(map[string][]string)
	}
	if key, ok := m.findCustomContextKey(name); ok {
		delete(m.CustomContexts, key)
	}
	m.CustomContexts[name] = projectIDs
}

// DeleteCustomContext mirrors (*Config).DeleteCustomContext, clearing the active
// custom-context filter when the deleted context was selected.
func (m *MachineState) DeleteCustomContext(name string) bool {
	key, ok := m.findCustomContextKey(name)
	if !ok {
		return false
	}
	delete(m.CustomContexts, key)
	if strings.EqualFold(m.FilterCustomContext, name) {
		m.FilterCustomContext = ""
	}
	return true
}

// CustomContextExists mirrors (*Config).CustomContextExists.
func (m *MachineState) CustomContextExists(name string) bool {
	_, ok := m.findCustomContextKey(name)
	return ok
}

// IsProjectInCustomContext mirrors (*Config).IsProjectInCustomContext.
func (m *MachineState) IsProjectInCustomContext(customContextName, projectID string) bool {
	return slices.Contains(m.GetCustomContext(customContextName), projectID)
}

// --- MachineState: recent task visits --------------------------------------

// GetRecentTaskVisits mirrors (*Config).GetRecentTaskVisits.
func (m *MachineState) GetRecentTaskVisits() []RecentTaskVisit {
	return m.RecentTaskVisits
}

// SetRecentTaskVisits mirrors (*Config).SetRecentTaskVisits.
func (m *MachineState) SetRecentTaskVisits(items []RecentTaskVisit) {
	m.RecentTaskVisits = items
}

// --- MachineState: command palette history and recents ---------------------

// GetRecentPaletteItems mirrors (*Config).GetRecentPaletteItems.
func (m *MachineState) GetRecentPaletteItems() []RecentPaletteItem {
	if m.RecentPaletteItems == nil {
		return []RecentPaletteItem{}
	}
	return m.RecentPaletteItems
}

// SetRecentPaletteItems mirrors (*Config).SetRecentPaletteItems.
func (m *MachineState) SetRecentPaletteItems(items []RecentPaletteItem) {
	m.RecentPaletteItems = items
}

// GetLastPaletteCategory mirrors (*Config).GetLastPaletteCategory.
func (m *MachineState) GetLastPaletteCategory() string {
	return m.LastPaletteCategory
}

// SetLastPaletteCategory mirrors (*Config).SetLastPaletteCategory.
func (m *MachineState) SetLastPaletteCategory(category string) {
	m.LastPaletteCategory = category
}

// GetPaletteSearchHistory mirrors (*Config).GetPaletteSearchHistory.
func (m *MachineState) GetPaletteSearchHistory() []string {
	if m.PaletteSearchHistory == nil {
		return []string{}
	}
	return m.PaletteSearchHistory
}

// SetPaletteSearchHistory mirrors (*Config).SetPaletteSearchHistory.
func (m *MachineState) SetPaletteSearchHistory(history []string) {
	m.PaletteSearchHistory = history
}

// GetGlobalSearchHistory mirrors (*Config).GetGlobalSearchHistory.
func (m *MachineState) GetGlobalSearchHistory() []string {
	if m.GlobalSearchHistory == nil {
		return []string{}
	}
	return m.GlobalSearchHistory
}

// SetGlobalSearchHistory mirrors (*Config).SetGlobalSearchHistory.
func (m *MachineState) SetGlobalSearchHistory(history []string) {
	m.GlobalSearchHistory = history
}

// GetCommandHistory mirrors (*Config).GetCommandHistory.
func (m *MachineState) GetCommandHistory() []string {
	if m.CommandHistory == nil {
		return []string{}
	}
	return m.CommandHistory
}

// AddCommandToHistory mirrors (*Config).AddCommandToHistory.
func (m *MachineState) AddCommandToHistory(name string) {
	if name == "" {
		return
	}
	const maxHistory = 10

	var newHistory []string
	for _, cmd := range m.CommandHistory {
		if cmd != name {
			newHistory = append(newHistory, cmd)
		}
	}

	m.CommandHistory = append([]string{name}, newHistory...)

	if len(m.CommandHistory) > maxHistory {
		m.CommandHistory = m.CommandHistory[:maxHistory]
	}
}

// --- MachineState: pinned commands -----------------------------------------

// GetPinnedCommands mirrors (*Config).GetPinnedCommands.
func (m *MachineState) GetPinnedCommands() []string {
	if m.PinnedCommands == nil {
		return []string{}
	}
	return m.PinnedCommands
}

// SetPinnedCommands mirrors (*Config).SetPinnedCommands.
func (m *MachineState) SetPinnedCommands(commands []string) {
	m.PinnedCommands = commands
}

// IsCommandPinned mirrors (*Config).IsCommandPinned.
func (m *MachineState) IsCommandPinned(name string) bool {
	return slices.Contains(m.PinnedCommands, name)
}

// TogglePinnedCommand mirrors (*Config).TogglePinnedCommand.
func (m *MachineState) TogglePinnedCommand(name string) bool {
	for i, cmd := range m.PinnedCommands {
		if cmd == name {
			m.PinnedCommands = append(m.PinnedCommands[:i], m.PinnedCommands[i+1:]...)
			return false
		}
	}
	m.PinnedCommands = append(m.PinnedCommands, name)
	return true
}

// --- MachineState: saved searches ------------------------------------------

// GetSavedSearches mirrors (*Config).GetSavedSearches.
func (m *MachineState) GetSavedSearches() map[string]string {
	if m.SavedSearches == nil {
		return make(map[string]string)
	}
	return m.SavedSearches
}

// SaveSearch mirrors (*Config).SaveSearch.
func (m *MachineState) SaveSearch(name, query string) {
	if m.SavedSearches == nil {
		m.SavedSearches = make(map[string]string)
	}
	m.SavedSearches[name] = query
}

// DeleteSavedSearch mirrors (*Config).DeleteSavedSearch.
func (m *MachineState) DeleteSavedSearch(name string) bool {
	if m.SavedSearches == nil {
		return false
	}
	if _, exists := m.SavedSearches[name]; exists {
		delete(m.SavedSearches, name)
		return true
	}
	return false
}

// GetSavedSearch mirrors (*Config).GetSavedSearch.
func (m *MachineState) GetSavedSearch(name string) string {
	if m.SavedSearches == nil {
		return ""
	}
	return m.SavedSearches[name]
}
