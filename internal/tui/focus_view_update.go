package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *FocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, func(s string) { m.setStatus(s) }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case FocusViewRefreshMsg:
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *FocusViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

func (m *FocusViewModel) handleQueueReorder(delta int) (tea.Model, tea.Cmd) {
	if m.tabs.Active != focusTabQueue {
		return m, nil
	}
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return m, nil
	}

	cur := m.filtered[m.cursor]
	if cur.TreePrefix != "" {
		return m, nil
	}

	target := m.cursor + delta
	for target >= 0 && target < len(m.filtered) && m.filtered[target].TreePrefix != "" {
		target += delta
	}
	if target < 0 || target >= len(m.filtered) {
		return m, nil
	}

	q := *m.queue
	fromQ, toQ := -1, -1
	targetID := m.filtered[target].Task.ID
	for i, ft := range q {
		if ft.Task.ID == cur.Task.ID {
			fromQ = i
		}
		if ft.Task.ID == targetID {
			toQ = i
		}
	}
	if fromQ < 0 || toQ < 0 {
		return m, nil
	}

	m.reorderQueue(fromQ, toQ)
	m.applyFilter()

	for i, ft := range m.filtered {
		if ft.Task.ID == cur.Task.ID {
			m.cursor = i
			break
		}
	}
	return m, nil
}

func (m *FocusViewModel) handleTabNavigation(delta int) (tea.Model, tea.Cmd) {
	if m.subTabRowFocused && len(m.subTabNames) > 1 {
		CycleTab(&m.subTab, len(m.subTabNames)+1, delta)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	}

	m.tabs.Cycle(delta)
	m.subTab = 0
	m.subTabRowFocused = false
	m.showDismissed = false
	m.recomputeSuggested()
	m.clampCursor()
	return m, nil
}

func (m *FocusViewModel) refreshViewState() {
	if m.tabs.Active == focusTabSuggested {
		m.showDismissed = false
		for id := range m.dismissedIDs {
			delete(m.dismissedIDs, id)
		}
		m.persistDismissed()
	}

	m.loadTasks()
	m.clampCursor()
	m.setStatus("Refreshed")
}
