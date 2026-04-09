package tui

import (
	tea "charm.land/bubbletea/v2"
)

// stageValidationClearMsg is sent after a timeout to clear the stage validation error.
type stageValidationClearMsg struct{}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stageValidationClearMsg:
		m.stageValidationErr = ""
		return m, nil
	case tea.KeyPressMsg:
		handled, cmd := m.handleKeyPress(msg)
		if handled {
			return m, cmd
		}
	}

	cmd := m.updateFocusedInput(msg)
	if m.tabs.Active == 1 {
		ApplyTheme(m.buildTentativeThemeConfig())
	}
	m.validateFocusedField()

	return m, cmd
}
