package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

// commonFilterView implementation for AllTasksViewModel.

func (m *AllTasksViewModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *AllTasksViewModel) getConfig() *config.Config       { return m.config }
func (m *AllTasksViewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *AllTasksViewModel) refreshAndClamp()                { m.applyFiltersAndGroup(); m.clampCursor() }
func (m *AllTasksViewModel) buildTagCmd() tea.Cmd {
	tags := buildAllTasksTagList(m)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewAllTasks, Title: "Filter by Tags"}
	}
}

// handleAllTasksFilterPrefix handles the second key after pressing F (filter prefix).
func (m *AllTasksViewModel) handleAllTasksFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys first
	switch key {
	case "d":
		m.filter.DueDate = (m.filter.DueDate + 1) % (DueDateNone + 1)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	case "P":
		m.toggleProjectFilter()
		return m, nil
	case "x":
		// Override common x to also clear project filter
		clearCommonFilters(&m.filter)
		m.projectFilter = make(map[string]bool)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	}

	// Common filter prefix keys
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleAllTasksFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *AllTasksViewModel) handleAllTasksFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}

// buildAllTasksTagList scans all tasks and builds a sorted list of tags with counts.
func buildAllTasksTagList(view *AllTasksViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, item := range view.allTasks {
		for _, tag := range item.Task.Tags {
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
