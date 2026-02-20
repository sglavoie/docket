package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
	tags := buildTagListFromTasks(tasks)
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

	task := row.Task
	if fuzzy.MatchTerms(task.Title, query) {
		return true
	}
	if fuzzy.MatchTerms(task.Source, query) {
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

	return false
}
