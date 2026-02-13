package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

// commonFilterView implementation for DailyReviewModel.

func (m *DailyReviewModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *DailyReviewModel) getConfig() *config.Config       { return m.config }
func (m *DailyReviewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *DailyReviewModel) refreshAndClamp()                { m.loadTasks(); m.clampCursor() }
func (m *DailyReviewModel) buildTagCmd() tea.Cmd {
	tags := buildDailyReviewTagList(m)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDailyReview, Title: "Filter by Tags"}
	}
}

// handleDailyReviewFilterPrefix handles the second key after pressing F (filter prefix) in daily review.
// Omits DueDate (sections group by due urgency) and Status (sections group by status).
func (m *DailyReviewModel) handleDailyReviewFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleDailyReviewFilterSubPrefix handles all filter sub-prefix keys (Ft, Fa, Fas-Fap, FS).
func (m *DailyReviewModel) handleDailyReviewFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
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
