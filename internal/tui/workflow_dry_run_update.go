package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *WorkflowDryRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case dryRunDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.results = msg.results
		m.conflicts = msg.conflicts
		m.ruleCount = msg.ruleCount
		m.taskCount = msg.taskCount
		m.filtered = m.results
		if m.singleRule != nil {
			m.statusLabel = "Dry Run: " + m.singleRule.Name
		} else {
			m.statusLabel = "Workflow Dry Run"
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.loading {
			return m, nil
		}

		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			_ = confirmed
			return m, nil
		}

		// Handle search mode
		if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterResults); handled {
			return model, cmd
		}

		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

		switch msg.String() {
		case "esc":
			if m.searchQuery != "" {
				m.ClearSearch()
				m.filterResults()
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "q":
			return m, func() tea.Msg { return BackMsg{} }

		case "/":
			return m, m.ActivateSearch()

		case "j", "down":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "g":
			m.pendingPrefix = "g"
			return m, nil

		case "G":
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, nil
		}
	}

	return m, nil
}
