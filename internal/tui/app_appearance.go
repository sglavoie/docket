package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/appearance"
)

// handleAppearanceResult switches the active theme when the OS appearance has
// changed. Returns a tea.Cmd for any needed refresh side-effects, or nil.
func (m *AppModel) handleAppearanceResult(msg systemAppearanceResultMsg) tea.Cmd {
	if msg.Appearance == appearance.Unknown {
		return nil
	}

	desired := string(msg.Appearance)
	if desired == m.config.ActiveTheme {
		return nil
	}

	// Don't switch while the user is editing palette colours in Settings.
	if m.currentView() == ViewSettings {
		return nil
	}

	m.config.ActiveTheme = desired
	ApplyTheme(NewThemeConfig(m.config))
	saveConfigWarn(m.configManager, m.config, "auto-theme sync")

	if m.dashboard != nil {
		m.dashboard.Refresh(m.config)
	}

	return nil
}
