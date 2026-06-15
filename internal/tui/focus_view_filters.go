package tui

import tea "charm.land/bubbletea/v2"

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *FocusViewModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *FocusViewModel) buildTagCmd() tea.Cmd {
	return buildTagCmdFromProjectTasks(
		projectsFromFocusTasks(m.tasks),
		tasksFromFocusTasks(m.tasks),
		m.filter.Tags,
		ViewFocusView,
		"Filter by Tags",
		false,
	)
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
