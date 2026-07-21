package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case presetDeletedMsg:
		m.themePresetFlash = fmt.Sprintf("deleted preset '%s'", msg.name)
		if msg.listEmpty {
			m.cancelPresetMode()
		} else {
			themes, _ := m.manager.ListThemes()
			m.themePresetList = themes
			if m.themePresetCursor >= len(themes) && m.themePresetCursor > 0 {
				m.themePresetCursor = len(themes) - 1
			}
		}
		flash := tea.Tick(2*time.Second, func(time.Time) tea.Msg { return presetFlashClearMsg{} })
		if msg.listEmpty {
			return m, tea.Batch(flash, func() tea.Msg {
				return settingsTransientOverlayMsg{Open: false, Kind: OverlayPicker, Key: settingsThemePresetOverlayKey}
			})
		}
		return m, flash
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
