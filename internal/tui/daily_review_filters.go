package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ShowDailyReviewTagFilterSelectorMsg requests opening the tag filter selector for daily review.
type ShowDailyReviewTagFilterSelectorMsg struct{}

// DailyReviewTagFilterSelectorDoneMsg is sent when the daily review tag filter selector completes.
type DailyReviewTagFilterSelectorDoneMsg struct {
	SelectedTags []string
	Canceled     bool
}

// handleDailyReviewFilterPrefix handles the second key after pressing F (filter prefix) in daily review.
// Omits DueDate (sections group by due urgency) and Status (sections group by status).
func (m *DailyReviewModel) handleDailyReviewFilterPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.filter.Priority == 1 {
			m.filter.Priority = 0
		} else {
			m.filter.Priority = 1
		}
		m.loadTasks()
		m.clampCursor()
		return m, nil
	case "t":
		m.pendingPrefix = "Ft"
		return m, nil
	case "#":
		return m, func() tea.Msg {
			return ShowDailyReviewTagFilterSelectorMsg{}
		}
	case "a":
		m.pendingPrefix = "Fa"
		return m, nil
	case "S":
		m.pendingPrefix = "FS"
		return m, nil
	case "x":
		clearCommonFilters(&m.filter)
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// handleDailyReviewFilterTypePrefix handles filter type sub-prefix.
func (m *DailyReviewModel) handleDailyReviewFilterTypePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterTypeKey(key, &m.filter) {
		m.loadTasks()
		m.clampCursor()
	}
	return m, nil
}

// handleDailyReviewFilterAttrPrefix handles the attribute filter sub-prefix.
func (m *DailyReviewModel) handleDailyReviewFilterAttrPrefix(key string) (tea.Model, tea.Cmd) {
	newPrefix, handled := handleFilterAttrKey(key, &m.filter)
	if !handled {
		return m, nil
	}
	if newPrefix != "" {
		m.pendingPrefix = newPrefix
		return m, nil
	}
	m.loadTasks()
	m.clampCursor()
	return m, nil
}

// handleDailyReviewFilterAttrSubPrefix handles attribute value sub-prefixes.
func (m *DailyReviewModel) handleDailyReviewFilterAttrSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
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
		m.loadTasks()
		m.clampCursor()
	}
	return m, nil
}

// handleDailyReviewFilterScorePrefix handles the score threshold sub-prefix.
func (m *DailyReviewModel) handleDailyReviewFilterScorePrefix(key string) (tea.Model, tea.Cmd) {
	if handleFilterScoreKey(key, &m.filter, m.config) {
		m.loadTasks()
		m.clampCursor()
	}
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *DailyReviewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *DailyReviewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *DailyReviewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}
