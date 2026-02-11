package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleDueDateFilterPrefix handles the second key after pressing F (filter prefix) in due date overview.
func (m *DueDateOverviewModel) handleDueDateFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.loadTasksWithDueDates()
		m.clampCursor()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		tags := buildDueDateTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDueDateOverview, Title: "Filter by Tags"}
		}
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.loadTasksWithDueDates()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleDueDateFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *DueDateOverviewModel) handleDueDateFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
		m.loadTasksWithDueDates()
		m.clampCursor()
	}
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *DueDateOverviewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *DueDateOverviewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *DueDateOverviewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}

// buildDueDateTagList scans all tasks in the due date groups and builds a sorted tag list.
func buildDueDateTagList(ddo *DueDateOverviewModel) []TagInfo {
	tagCounts := make(map[string]int)
	groups := [][]DueDateTask{ddo.overdue, ddo.today, ddo.thisWeek, ddo.later}
	for _, group := range groups {
		for _, dt := range group {
			for _, tag := range dt.Task.Tags {
				tagCounts[tag]++
			}
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
