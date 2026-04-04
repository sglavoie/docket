package tui

import (
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *FocusBoardModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *FocusBoardModel) buildTagCmd() tea.Cmd {
	tasks := make([]*models.Task, len(m.rows))
	for i := range m.rows {
		tasks[i] = m.rows[i].Task
	}
	return buildTagCmdFromTasks(tasks, m.filter.Tags, ViewFocusBoard, "Filter by Tags")
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
	if m.pendingPrefix == "" {
		return nil
	}
	var currentStage string
	var currentPriority int
	if row := m.getRowAtCursor(); row != nil {
		currentStage = row.Task.Stage
		currentPriority = row.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["g"] = FocusBoardNavigationPrefixHints
	hints["z"] = []PrefixHint{
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true))
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// FocusBoardNavigationPrefixHints contains "g" prefix hints for focus board.
var FocusBoardNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to task in project tree"},
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
