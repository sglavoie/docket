package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *FocusBoardModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *FocusBoardModel) buildTagCmd() tea.Cmd {
	// Use allRows (all non-complete, non-snoozed) so the tag picker includes
	// tags from tasks visible on any tab, not just active/due-soon ones.
	return buildTagCmdFromProjectTasks(
		projectsFromBoardRows(m.allRows),
		tasksFromBoardRows(m.allRows),
		m.filter.Tags,
		ViewFocusBoard,
		"Filter by Tags",
		false,
	)
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

// ActiveFilterCount returns the number of active filters. Overrides FilterMixin
// to include the search query in the count.
func (m *FocusBoardModel) ActiveFilterCount() int {
	count := m.FilterMixin.ActiveFilterCount()
	if m.searchQuery != "" {
		count++
	}
	return count
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
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	var hints []PrefixHint
	for _, h := range m.Dispatcher.PrefixHints() {
		hints = append(hints, PrefixHint{Key: h.Key, Description: h.Description})
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  PrefixHints{prefix: hints},
	}
}

// matchesSearch returns true if the board row matches the search query.
func (m *FocusBoardModel) matchesSearch(row boardRow) bool {
	if TaskMatchesSearch(row.Task, m.searchQuery) {
		return true
	}
	query := strings.TrimSpace(m.searchQuery)
	if query == "" {
		return true
	}
	if fuzzy.MatchTerms(row.Task.Source, query) {
		return true
	}
	if row.Project != nil && fuzzy.MatchTerms(row.Project.Name, query) {
		return true
	}
	return false
}
