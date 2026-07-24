package tui

import tea "charm.land/bubbletea/v2"

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
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix("Fv")
		} else {
			m.pendingPrefix = "Fv"
		}
		return m, nil
	case "d":
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix("Fd")
		} else {
			m.pendingPrefix = "Fd"
		}
		return m, nil
	case "c":
		target := ModeComplete
		if m.activePerspective == PerspectiveCompleted {
			target = ModeSnoozed
		}
		m.resetViewScrollOffset()
		return m.selectOutlinerMode(target)
	case "@":
		// Single-project instance filters its own project; the canonical cross
		// Outliner filters the scoped project when scoped, or across every loaded
		// project when global — so F @ works in both global and project scopes.
		if m.project != nil {
			return m, func() tea.Msg {
				return ShowSectionFilterSelectorMsg{Project: m.project}
			}
		}
		projects := m.scopeProjects()
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{Projects: projects}
		}
	case "r":
		m.filter.Actionable = !m.filter.Actionable
		m.resetViewScrollOffset()
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
		m.resetViewScrollOffset()
		m.rebuildRows()
		return m, func() tea.Msg { return outlinerFilterStateChangedMsg{} }
	}

	// Common filter prefix keys
	tagSource := ViewOutliner
	if m.project == nil {
		tagSource = ViewCrossProjectOutliner
	}
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		if m.project != nil {
			return buildTagCmdFromProject(m.project, m.filter.Tags, tagSource, "Filter by Tags", false)
		}
		return buildTagCmdFromProjectTasks(
			m.loadedProjects,
			m.crossMergedTaskSlice(),
			m.filter.Tags,
			tagSource,
			"Filter by Tags",
			false,
		)
	})
	setPfx := func(p string) {
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix(p)
		} else {
			m.pendingPrefix = p
		}
	}
	if cmd := applyCommonFilterResult(result, setPfx, func() {
		m.resetViewScrollOffset()
		m.rebuildRows()
	}); cmd != nil {
		return m, cmd
	}
	if result.Changed {
		return m, nil
	}
	return m, nil
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
