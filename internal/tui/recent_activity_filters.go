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

// handleRecentActivityFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *RecentActivityModel) handleRecentActivityFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
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
