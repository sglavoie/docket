package tui

import (
	"github.com/sglavoie/tt/internal/models"

	tea "charm.land/bubbletea/v2"
)

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *OutlinerModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Any manual filter change clears the saved filter indicator
	if key != "v" {
		m.clearActiveSavedFilter()
	}

	// View-specific filter keys
	switch key {
	case "P":
		if m.project != nil {
			return m, nil // single-project: no-op
		}
		selected := m.filterProjects
		projects := m.loadedProjects
		return m, func() tea.Msg {
			return ShowProjectFilterSelectorMsg{
				Projects: projects,
				Selected: selected,
			}
		}
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "c":
		if m.activePerspective == PerspectiveCompleted {
			m.activePerspective = PerspectiveSnoozed
		} else {
			m.activePerspective = PerspectiveCompleted
		}
		m.rebuildRows()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Project: m.project}
		}
	case "r":
		m.filter.Actionable = !m.filter.Actionable
		m.rebuildRows()
		return m, nil
	case "D":
		source := ViewOutliner
		if m.project == nil {
			source = ViewCrossProjectOutliner
		}
		return m, func() tea.Msg {
			return ShowColumnPickerMsg{Source: source, ColumnConfig: m.columnConfig}
		}
	case "x":
		clearCommonFilters(&m.filter.CommonFilterState)
		m.filter.Actionable = false
		m.filter.Sections = make(map[string]bool)
		m.filterProjects = make(map[string]bool)
		m.ClearSearch()
		m.rebuildRows()
		return m, func() tea.Msg { return outlinerFilterStateChangedMsg{} }
	}

	// Common filter prefix keys
	tagSource := ViewOutliner
	if m.project == nil {
		tagSource = ViewCrossProjectOutliner
	}
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		var taskSlice []*models.Task
		if m.project != nil {
			taskSlice = m.project.Tasks
		} else {
			taskSlice = m.crossMergedTaskSlice()
		}
		tags := BuildTagList(taskSlice, false)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: tagSource, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, func() { m.rebuildRows() }); cmd != nil {
		return m, cmd
	}
	if result.Changed {
		return m, nil
	}
	return m, nil
}

// handleSavedFilterPrefix handles the third key after pressing Fv (saved filters sub-prefix).
func (m *OutlinerModel) handleSavedFilterPrefix(key string) (tea.Model, tea.Cmd) {
	cmd, _ := handleSavedFilterPrefixKey(key)
	return m, cmd
}

// cycleDueDateFilter cycles through due date filter modes.
func (m *OutlinerModel) cycleDueDateFilter() {
	switch m.filter.DueDate {
	case DueDateAll:
		m.filter.DueDate = DueDateOverdue
	case DueDateOverdue:
		m.filter.DueDate = DueDateToday
	case DueDateToday:
		m.filter.DueDate = DueDateThisWeek
	case DueDateThisWeek:
		m.filter.DueDate = DueDateNone
	default:
		m.filter.DueDate = DueDateAll
	}
}
