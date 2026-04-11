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
func (m *FocusViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	m.err = nil
	m.clearStatus()

	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return taskOpsDeleteTask(m)
		}
		return m, nil
	}

	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmct.", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		RunEscCascade(m.escSteps())
		return m, nil
	case "enter":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project}
			}
		}
		return m, nil
	case "J":
		return m.handleQueueReorder(1)
	case "K":
		return m.handleQueueReorder(-1)
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil
	case "[", "h":
		return m.handleTabNavigation(-1)
	case "]", "l":
		return m.handleTabNavigation(1)
	case "r":
		m.refreshViewState()
		return m, nil
	case "1", "2", "3", "4", "5":
		return taskOpsChangePriority(m, key)
	case "D":
		if m.tabs.Active == focusTabSuggested {
			m.showDismissed = !m.showDismissed
			m.recomputeSuggested()
			m.clampCursor()
			return m, nil
		}
	}

	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		if pfx == "z" && k == "p" {
			m.previewMode = !m.previewMode
			return m, nil, true
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
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
