package tui

import tea "github.com/charmbracelet/bubbletea"

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if m.zoomDepth == 0 {
			return m, func() tea.Msg {
				return CloseOutlinerMsg{}
			}
		}
		m.zoomDepth--
		return m, nil

	case "esc":
		if m.multiSelect {
			m.multiSelect = false
			return m, nil
		}
		return m, nil

	// Cursor movement
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "g":
		m.cursor = 0
		return m, nil
	case "G":
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		return m, nil
	case "ctrl+d", "pgdown":
		half := max(m.contentHeight()/2, 1)
		m.cursor += half
		m.clampCursor()
		return m, nil
	case "ctrl+u", "pgup":
		half := max(m.contentHeight()/2, 1)
		m.cursor -= half
		m.clampCursor()
		return m, nil

	// View switching
	case "ctrl+p":
		return m, func() tea.Msg {
			return ShowProjectTreeMsg{Project: m.project}
		}

	case "ctrl+k":
		return m, func() tea.Msg {
			return ShowKanbanMsg{Project: m.project}
		}

	case "ctrl+f":
		return m, func() tea.Msg {
			return ShowFocusBoardMsg{}
		}
	}

	return m, nil
}

// ShouldCaptureEsc returns true so the outliner handles q/Esc internally
// for zoom-level navigation and transient mode dismissal.
func (m *OutlinerModel) ShouldCaptureEsc() bool {
	return true
}

// HasTextInput returns true when the outliner has an active text input field.
func (m *OutlinerModel) HasTextInput() bool {
	return false
}
