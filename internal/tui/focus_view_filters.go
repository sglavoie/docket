package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

// handleFocusFilterPrefix handles the second key after pressing F (filter prefix) in focus view.
func (m *FocusViewModel) handleFocusFilterPrefix(key string) (tea.Model, tea.Cmd) {
	refresh := func() { m.applyFilter(); m.clampCursor() }
	result := handleCommonFilterPrefix(key, &m.filter, func() tea.Cmd {
		tags := buildFocusTagList(m)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewFocusView, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, refresh); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// handleFocusFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *FocusViewModel) handleFocusFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	refresh := func() { m.applyFilter(); m.clampCursor() }
	if dispatchFilterSubPrefix(prefix, key, &m.filter, m.config, func(p string) { m.pendingPrefix = p }) {
		refresh()
	}
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *FocusViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *FocusViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *FocusViewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}

// buildFocusTagList scans all focus tasks and builds a sorted list of tags with counts.
func buildFocusTagList(focus *FocusViewModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, ft := range focus.tasks {
		for _, tag := range ft.Task.Tags {
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
