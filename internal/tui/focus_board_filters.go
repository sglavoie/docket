package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
)

// commonFilterView implementation for FocusBoardModel.

func (m *FocusBoardModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *FocusBoardModel) getConfig() *config.Config       { return m.config }
func (m *FocusBoardModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *FocusBoardModel) refreshAndClamp()                { m.applyFilter(); m.clampCursor() }
func (m *FocusBoardModel) buildTagCmd() tea.Cmd {
	tags := buildBoardTagList(m)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewFocusBoard, Title: "Filter by Tags"}
	}
}

// handleFocusBoardFilterPrefix handles the second key after pressing F (filter prefix).
func (m *FocusBoardModel) handleFocusBoardFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFocusBoardFilterSubPrefix handles all filter sub-prefix keys.
func (m *FocusBoardModel) handleFocusBoardFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *FocusBoardModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter) || m.searchQuery != ""
}

// ClearAllFilters resets all filters to their default state.
func (m *FocusBoardModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
	m.searchQuery = ""
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *FocusBoardModel) renderActiveFilterIndicator() string {
	parts := commonFilterParts(&m.filter)
	if m.searchQuery != "" {
		parts = append(parts, "search:"+m.searchQuery)
	}
	return renderFilterBadge(parts)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *FocusBoardModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// matchesSearch returns true if the board row matches the search query.
func (m *FocusBoardModel) matchesSearch(row boardRow) bool {
	if m.searchQuery == "" {
		return true
	}
	query := strings.TrimSpace(m.searchQuery)

	if row.isBoardItem() {
		item := row.BoardItem
		if fuzzy.MatchTerms(item.Title, query) {
			return true
		}
		if fuzzy.MatchTerms(item.Source, query) {
			return true
		}
		for _, tag := range item.Tags {
			if fuzzy.MatchTerms(tag, query) {
				return true
			}
		}
		if fuzzy.MatchTerms(item.ProjectName, query) {
			return true
		}
	}

	if row.isTask() {
		task := row.Task
		if fuzzy.MatchTerms(task.Title, query) {
			return true
		}
		for _, tag := range task.Tags {
			if fuzzy.MatchTerms(tag, query) {
				return true
			}
		}
		if fuzzy.MatchTerms(task.Description, query) {
			return true
		}
		if row.Project != nil && fuzzy.MatchTerms(row.Project.Name, query) {
			return true
		}
	}

	return false
}

// boardItemPassesFilters checks if a board item passes the active common filters.
// Only priority and tags are relevant for board items; other filters are task-specific.
func boardItemPassesFilters(row boardRow, f *CommonFilterState) bool {
	item := row.BoardItem
	if f.Priority == 1 && item.Priority > 2 {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			found := false
			for _, t := range item.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	// Type/status/dueDate/attrs/score filters don't apply to board items.
	// If any of these are active, exclude board items since they can't match.
	if f.Type != "" || f.Status != "" || f.DueDate != DueDateAll || len(f.Attrs) > 0 || f.ScoreMin > 0 {
		return false
	}
	return true
}

// buildBoardTagList scans all board items and aggregated tasks for tags.
func buildBoardTagList(m *FocusBoardModel) []TagInfo {
	tagCounts := make(map[string]int)
	for _, r := range m.rows {
		if r.isBoardItem() {
			for _, tag := range r.BoardItem.Tags {
				tagCounts[tag]++
			}
		}
		if r.isTask() {
			for _, tag := range r.Task.Tags {
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
