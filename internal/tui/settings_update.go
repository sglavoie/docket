package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// stageValidationClearMsg is sent after a timeout to clear the stage validation error.
type stageValidationClearMsg struct{}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stageValidationClearMsg:
		m.stageValidationErr = ""
		return m, nil
	case presetDeletedMsg:
		m.themePresetFlash = fmt.Sprintf("deleted preset '%s'", msg.name)
		if msg.listEmpty {
			m.cancelPresetMode()
		} else {
			themes, _ := m.configManager.ListThemes()
			m.themePresetList = themes
			if m.themePresetCursor >= len(themes) && m.themePresetCursor > 0 {
				m.themePresetCursor = len(themes) - 1
			}
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return presetFlashClearMsg{}
		})
	case presetFlashClearMsg:
		m.themePresetFlash = ""
		return m, nil
	case tea.KeyPressMsg:
		handled, cmd := m.handleKeyPress(msg)
		if handled {
			return m, cmd
		}
	}

	cmd := m.updateFocusedInput(msg)
	if m.tabs.Active == tabPalette {
		ApplyTheme(m.buildTentativeThemeConfig())
	}
	m.validateFocusedField()

	return m, cmd
}
