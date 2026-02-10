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

// handleDueDateFilterTypePrefix handles filter type sub-prefix.
func (m *DueDateOverviewModel) handleDueDateFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterTypeKey(key, &m.filter) {
		m.loadTasksWithDueDates()
		m.clampCursor()
	}
	return m, nil
}

// handleDueDateFilterStatusPrefix handles filter status sub-prefix.
func (m *DueDateOverviewModel) handleDueDateFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterStatusKey(key, &m.filter) {
		m.loadTasksWithDueDates()
		m.clampCursor()
	}
	return m, nil
}

// handleDueDateFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *DueDateOverviewModel) handleDueDateFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.loadTasksWithDueDates()
	m.clampCursor()
	return m, nil
}

// handleDueDateFilterAttrSubPrefix handles attribute value sub-prefixes.
func (m *DueDateOverviewModel) handleDueDateFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
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
		m.loadTasksWithDueDates()
		m.clampCursor()
	}
	return m, nil
}

// handleDueDateFilterScorePrefix handles the score threshold sub-prefix.
func (m *DueDateOverviewModel) handleDueDateFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterScoreKey(key, &m.filter, m.config) {
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
