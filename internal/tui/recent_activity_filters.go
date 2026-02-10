package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleRecentActivityFilterPrefix handles the second key after pressing F (filter prefix) in recent activity.
func (m *RecentActivityModel) handleRecentActivityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		tags := buildRecentActivityTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewRecentActivity, Title: "Filter by Tags"}
		}
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleRecentActivityFilterTypePrefix handles filter type sub-prefix.
func (m *RecentActivityModel) handleRecentActivityFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterTypeKey(key, &m.filter) {
		m.applyFilter()
		m.clampCursor()
	}
	return m, nil
}

// handleRecentActivityFilterStatusPrefix handles filter status sub-prefix.
func (m *RecentActivityModel) handleRecentActivityFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterStatusKey(key, &m.filter) {
		m.applyFilter()
		m.clampCursor()
	}
	return m, nil
}

// handleRecentActivityFilterDuePrefix handles filter due-date sub-prefix.
func (m *RecentActivityModel) handleRecentActivityFilterDuePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterDueDateKey(key, &m.filter) {
		m.applyFilter()
		m.clampCursor()
	}
	return m, nil
}

// handleRecentActivityFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *RecentActivityModel) handleRecentActivityFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.applyFilter()
	m.clampCursor()
	return m, nil
}

// handleRecentActivityFilterAttrSubPrefix handles attribute value sub-prefixes.
func (m *RecentActivityModel) handleRecentActivityFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	var handled bool
	switch prefix {
	case "Fas":
		handled = handleFilterAttrSeverityKey(key, &m.filter)
	case "Far":
		handled = handleFilterAttrReproducibleKey(key, &m.filter)
	case "Fae":
		handled = handleFilterAttrEffortKey(key, &m.filter)
	case "Fac":
		handled = handleFilterAttrCategoryKey(key, &m.filter)
	case "Fav":
		handled = handleFilterAttrValueKey(key, &m.filter)
	case "Faw":
		handled = handleFilterAttrWorktypeKey(key, &m.filter)
	case "Fao":
		handled = handleFilterAttrScopeKey(key, &m.filter)
	case "Fap":
		handled = handleFilterAttrPointsKey(key, &m.filter)
	}
	if handled {
		m.applyFilter()
		m.clampCursor()
	}
	return m, nil
}

// handleRecentActivityFilterScorePrefix handles the score threshold sub-prefix.
func (m *RecentActivityModel) handleRecentActivityFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterScoreKey(key, &m.filter, m.config) {
		m.applyFilter()
		m.clampCursor()
	}
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *RecentActivityModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *RecentActivityModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *RecentActivityModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}

// buildRecentActivityTagList scans all tasks and builds a sorted tag list.
func buildRecentActivityTagList(ra *RecentActivityModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, dt := range ra.tasks {
		for _, tag := range dt.Task.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}
