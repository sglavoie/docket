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

	q := *m.queue
	to := m.cursor + delta
	if m.cursor < 0 || m.cursor >= len(q) || to < 0 || to >= len(q) {
		return m, nil
	}

	m.reorderQueue(m.cursor, to)
	m.cursor = to
	m.applyFilter()
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
