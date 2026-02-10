package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleAllTasksFilterPrefix handles the second key after pressing F (filter prefix).
func (m *AllTasksViewModel) handleAllTasksFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "s":
		m.pendingPrefix = "Fs"
		return m, nil
	case "#":
		tags := buildAllTasksTagList(m)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewAllTasks, Title: "Filter by Tags"}
		}
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "d":
		// Cycle due date filter
		m.filter.DueDate = (m.filter.DueDate + 1) % (DueDateNone + 1)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	case "P":
		m.toggleProjectFilter()
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.projectFilter = make(map[string]bool)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleAllTasksFilterTypePrefix handles filter type sub-prefix.
func (m *AllTasksViewModel) handleAllTasksFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterTypeKey(key, &m.filter) {
		m.applyFiltersAndGroup()
		m.clampCursor()
	}
	return m, nil
}

// handleAllTasksFilterStatusPrefix handles filter status sub-prefix.
func (m *AllTasksViewModel) handleAllTasksFilterStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterStatusKey(key, &m.filter) {
		m.applyFiltersAndGroup()
		m.clampCursor()
	}
	return m, nil
}

// handleAllTasksFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *AllTasksViewModel) handleAllTasksFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.applyFiltersAndGroup()
	m.clampCursor()
	return m, nil
}

// handleAllTasksFilterAttrSubPrefix handles attribute value sub-prefixes.
func (m *AllTasksViewModel) handleAllTasksFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
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
		m.applyFiltersAndGroup()
		m.clampCursor()
	}
	return m, nil
}

// handleAllTasksFilterScorePrefix handles the score threshold sub-prefix.
func (m *AllTasksViewModel) handleAllTasksFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterScoreKey(key, &m.filter, m.config) {
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
