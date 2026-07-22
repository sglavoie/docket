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

// DeleteCustomContext removes a named custom context.
func (m *MachineState) DeleteCustomContext(name string) bool {
	key, ok := m.findCustomContextKey(name)
	if !ok {
		return false
	}
	delete(m.CustomContexts, key)
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

// --- MachineState: Home layout ---------------------------------------------

// GetHomeProjectsHeightPct returns the machine-local Home Projects/Attention
// split percentage. It defaults to DefaultHomeProjectsHeightPct for nil state or
// an unset value and clamps any persisted value into
// [MinHomeProjectsHeightPct, MaxHomeProjectsHeightPct] before a renderer uses it.
func (m *MachineState) GetHomeProjectsHeightPct() int {
	if m == nil || m.HomeProjectsHeightPct == nil {
		return DefaultHomeProjectsHeightPct
	}
	switch v := *m.HomeProjectsHeightPct; {
	case v < MinHomeProjectsHeightPct:
		return MinHomeProjectsHeightPct
	case v > MaxHomeProjectsHeightPct:
		return MaxHomeProjectsHeightPct
	default:
		return v
	}
}
