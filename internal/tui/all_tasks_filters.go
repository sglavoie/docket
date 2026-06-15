package tui

import tea "charm.land/bubbletea/v2"

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *AllTasksViewModel) refreshAndClamp() { m.applyFiltersAndGroup(); m.clampCursor() }

func (m *AllTasksViewModel) buildTagCmd() tea.Cmd {
	return buildTagCmdFromProjectTasks(
		projectsFromProjectCache(m.projectCache),
		tasksFromCrossProjectTasks(m.allTasks),
		m.filter.Tags,
		ViewAllTasks,
		"Filter by Tags",
		false,
	)
}

// handleAllTasksFilterPrefix handles the second key after pressing F (filter prefix).
func (m *AllTasksViewModel) handleAllTasksFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// View-specific keys first
	switch key {
	case "d":
		m.filter.DueDate = (m.filter.DueDate + 1) % (DueDateNone + 1)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	case "P":
		m.toggleProjectFilter()
		return m, nil
	case "x":
		// Override common x to also clear project filter
		clearCommonFilters(&m.filter)
		m.projectFilter = make(map[string]bool)
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil
	}

	// Common filter prefix keys
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleAllTasksFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *AllTasksViewModel) handleAllTasksFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
