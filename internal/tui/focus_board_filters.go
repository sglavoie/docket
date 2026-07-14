package tui

import (
	"maps"
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
	hints := PrefixHints{}
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.runtime.Machine(), true, true, false))
	if _, ok := hints[prefix]; !ok {
		fallback := make([]PrefixHint, 0, len(m.Dispatcher.PrefixHints()))
		for _, h := range m.Dispatcher.PrefixHints() {
			fallback = append(fallback, PrefixHint{Key: h.Key, Description: h.Description})
		}
		hints[prefix] = fallback
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  hints,
	}
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *FocusBoardModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, FS, F.).
func (m *FocusBoardModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
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
