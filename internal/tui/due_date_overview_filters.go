package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// commonFilterView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *DueDateOverviewModel) getConfig() *config.Config       { return m.config }
func (m *DueDateOverviewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *DueDateOverviewModel) refreshAndClamp()                { m.loadTasksWithDueDates(); m.clampCursor() }
func (m *DueDateOverviewModel) buildTagCmd() tea.Cmd {
	groups := [][]DueDateTask{m.overdue, m.today, m.thisWeek, m.later}
	var tasks []*models.Task
	for _, group := range groups {
		for i := range group {
			tasks = append(tasks, group[i].Task)
		}
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewDueDateOverview, Title: "Filter by Tags"}
	}
}

// handleDueDateFilterPrefix handles the second key after pressing F (filter prefix) in due date overview.
func (m *DueDateOverviewModel) handleDueDateFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleDueDateFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *DueDateOverviewModel) handleDueDateFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}

// HasActiveFilters returns true if any filter is currently active.
func (m *DueDateOverviewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter)
}

// ClearAllFilters resets all filters to their default state.
func (m *DueDateOverviewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *DueDateOverviewModel) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&m.filter)
}
