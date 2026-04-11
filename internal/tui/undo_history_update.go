package tui

import tea "charm.land/bubbletea/v2"

func (m *UndoHistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)

	case UndoHistoryRefreshMsg:
		m.reloadEntries()

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m *UndoHistoryModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showingDetail {
		return m.handleDetailKeyPress(msg), nil
	}

	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix == "g" {
		return m.handlePendingPrefix(msg), nil
	}

	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return CloseUndoHistoryMsg{} }
	case "j", "down":
		m.moveDown()
	case "k", "up":
		m.moveUp()
	case "h", "left":
		m.scrollDiffLeft()
	case "l", "right":
		m.scrollDiffRight()
	case "tab":
		m.focusNextPane()
	case "enter":
		return m.handleEnter()
	case "g":
		m.pendingPrefix = "g"
		return m, nil
	case "G":
		m.jumpToBottom()
	}

	return m, nil
}

func (m *UndoHistoryModel) handleDetailKeyPress(msg tea.KeyPressMsg) tea.Model {
	switch msg.String() {
	case "esc", "enter":
		m.showingDetail = false
	}

	return m
}

func (m *UndoHistoryModel) handlePendingPrefix(msg tea.KeyPressMsg) tea.Model {
	m.pendingPrefix = ""
	if msg.String() != "g" {
		return m
	}

	if m.focusedPane == 0 {
		m.cursor = 0
		m.computeDiff()
		m.resetDiffSelection(true)
		return m
	}

	m.diffScroll = 0
	m.diffCursor = 0
	return m
}

func (m *UndoHistoryModel) moveDown() {
	if m.focusedPane == 0 {
		if m.cursor < m.totalEntries()-1 {
			m.cursor++
			m.computeDiff()
			m.resetDiffSelection(true)
		}
		return
	}

	if m.diffCursor < len(m.diffLines)-1 {
		m.diffCursor++
		m.ensureDiffCursorVisible()
	}
}

func (m *UndoHistoryModel) moveUp() {
	if m.focusedPane == 0 {
		if m.cursor > 0 {
			m.cursor--
			m.computeDiff()
			m.resetDiffSelection(true)
		}
		return
	}

	if m.diffCursor > 0 {
		m.diffCursor--
		m.ensureDiffCursorVisible()
	}
}

func (m *UndoHistoryModel) scrollDiffLeft() {
	if m.focusedPane != 1 {
		return
	}

	m.diffHScroll = max(m.diffHScroll-10, 0)
}

func (m *UndoHistoryModel) scrollDiffRight() {
	if m.focusedPane == 1 {
		m.diffHScroll += 10
	}
}

func (m *UndoHistoryModel) focusNextPane() {
	m.focusedPane = (m.focusedPane + 1) % 2
	if m.focusedPane == 1 {
		m.diffHScroll = 0
	}
}

func (m *UndoHistoryModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.focusedPane == 1 && len(m.diffLines) > 0 && m.diffCursor < len(m.diffLines) {
		m.showingDetail = true
		m.detailLine = m.diffLines[m.diffCursor]
		return m, nil
	}

	if m.focusedPane == 0 && m.totalEntries() > 0 {
		if m.isOnRedoSide() {
			// Only the most recent redo entry (index 0) can be applied
			if m.redoIndex() != 0 {
				return m, func() tea.Msg { return InfoStatusMsg("Only the most recent redo entry can be applied") }
			}
			return m, m.performRedo()
		}
		// Only the most recent undo entry (first after redo entries) can be applied
		if m.undoIndex() != 0 {
			return m, func() tea.Msg { return InfoStatusMsg("Only the most recent undo entry can be applied") }
		}
		return m, m.performUndo()
	}

	return m, nil
}

func (m *UndoHistoryModel) jumpToBottom() {
	if m.focusedPane == 0 {
		if m.totalEntries() > 0 {
			m.cursor = m.totalEntries() - 1
			m.computeDiff()
			m.resetDiffSelection(true)
		}
		return
	}

	if len(m.diffLines) > 0 {
		m.diffCursor = len(m.diffLines) - 1
		m.ensureDiffCursorVisible()
	}
}

func (m *UndoHistoryModel) resetDiffSelection(resetHorizontal bool) {
	m.diffScroll = 0
	m.diffCursor = 0
	if resetHorizontal {
		m.diffHScroll = 0
	}
}
