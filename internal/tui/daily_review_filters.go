package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleDailyReviewFilterPrefix handles the second key after pressing F (filter prefix) in daily review.
// Omits DueDate (sections group by due urgency) and Status (sections group by status).
func (m *DailyReviewModel) handleDailyReviewFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.loadTasks()
		m.clampCursor()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "#":
		tags := buildDailyReviewTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDailyReview, Title: "Filter by Tags"}
		}
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleDailyReviewFilterSubPrefix handles all filter sub-prefix keys (Ft, Fa, Fas-Fap, FS).
func (m *DailyReviewModel) handleDailyReviewFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
		m.loadTasks()
		m.clampCursor()
	}
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *DailyReviewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *DailyReviewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *DailyReviewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}

// buildDailyReviewTagList scans all tasks in all review sections and builds a sorted tag list.
func buildDailyReviewTagList(dr *DailyReviewModel) []TagInfo {
	tagCounts := make(map[string]int)
	sections := [][]DailyReviewTask{
		dr.overdue, dr.dueToday, dr.dueSoon, dr.dueLater,
		dr.future, dr.inProgress, dr.blocked, dr.inReview, dr.old, dr.completed,
	}
	for _, section := range sections {
		for _, dt := range section {
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
