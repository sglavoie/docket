package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// commonFilterView implementation for FocusViewModel.

func (m *FocusViewModel) getFilter() *CommonFilterState   { return &m.filter }
func (m *FocusViewModel) getConfig() *config.Config       { return m.config }
func (m *FocusViewModel) setFilterPendingPrefix(p string) { m.pendingPrefix = p }
func (m *FocusViewModel) refreshAndClamp()                { m.applyFilter(); m.clampCursor() }
func (m *FocusViewModel) buildTagCmd() tea.Cmd {
	tasks := make([]*models.Task, len(m.tasks))
	for i := range m.tasks {
		tasks[i] = m.tasks[i].Task
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewFocusView, Title: "Filter by Tags"}
	}
}

// handleFocusFilterPrefix handles the second key after pressing F (filter prefix) in focus view.
func (m *FocusViewModel) handleFocusFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFocusFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *FocusViewModel) handleFocusFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
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
