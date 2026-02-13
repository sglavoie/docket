package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

// commonFilterView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *DueDateOverviewModel) getConfig() *config.Config       { return m.config }
func (m *DueDateOverviewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *DueDateOverviewModel) refreshAndClamp()                { m.loadTasksWithDueDates(); m.clampCursor() }
func (m *DueDateOverviewModel) buildTagCmd() tea.Cmd {
	tags := buildDueDateTagList(m)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDueDateOverview, Title: "Filter by Tags"}
	}
}

// handleDueDateFilterPrefix handles the second key after pressing F (filter prefix) in due date overview.
func (m *DueDateOverviewModel) handleDueDateFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleDueDateFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *DueDateOverviewModel) handleDueDateFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
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
