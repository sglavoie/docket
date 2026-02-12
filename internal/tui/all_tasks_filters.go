package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleAllTasksFilterPrefix handles the second key after pressing F (filter prefix).
func (m *AllTasksViewModel) handleAllTasksFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys
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
	result := handleCommonFilterPrefix(key, &m.filter, func() tea.Cmd {
		tags := buildAllTasksTagList(m)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewAllTasks, Title: "Filter by Tags"}
		}
	})
	if result.NewPrefix != "" {
		m.pendingPrefix = result.NewPrefix
	}
	if result.Changed {
		m.applyFiltersAndGroup()
		m.clampCursor()
	}
	if result.TagCmd != nil {
		return m, result.TagCmd
	}
	return m, nil
}

// handleAllTasksFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *AllTasksViewModel) handleAllTasksFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
		m.applyFiltersAndGroup()
		m.clampCursor()
	}
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
