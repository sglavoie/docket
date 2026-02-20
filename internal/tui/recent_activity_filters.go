package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// commonFilterView implementation for RecentActivityModel.

func (m *RecentActivityModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *RecentActivityModel) getConfig() *config.Config       { return m.config }
func (m *RecentActivityModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *RecentActivityModel) refreshAndClamp()                { m.applyFilter(); m.clampCursor() }
func (m *RecentActivityModel) buildTagCmd() tea.Cmd {
	tasks := make([]*models.Task, len(m.tasks))
	for i := range m.tasks {
		tasks[i] = m.tasks[i].Task
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewRecentActivity, Title: "Filter by Tags"}
	}
}

// handleRecentActivityFilterPrefix handles the second key after pressing F (filter prefix) in recent activity.
func (m *RecentActivityModel) handleRecentActivityFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific: "d" opens the due-date sub-prefix
	if key == "d" {
		m.pendingPrefix = "Fd"
		return m, nil
	}

	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleRecentActivityFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *RecentActivityModel) handleRecentActivityFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *RecentActivityModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *RecentActivityModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *RecentActivityModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}
