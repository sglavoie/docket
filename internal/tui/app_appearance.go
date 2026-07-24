package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/appearance"
	"github.com/sglavoie/tt/internal/logging"
)

// handleAppearanceResult switches the active theme when the OS appearance has
// changed. Returns a tea.Cmd for any needed refresh side-effects, or nil.
func (m *AppModel) handleAppearanceResult(msg systemAppearanceResultMsg) tea.Cmd {
	if msg.Appearance == appearance.Unknown {
		return nil
	}

	desired := string(msg.Appearance)
	if desired == m.runtime.Settings().ActiveTheme {
		return nil
	}

	// Don't switch while the user is editing palette colors in Settings.
	if m.currentView() == ViewSettings {
		return nil
	}

	// Persist the theme flip through the settings domain (not a whole-config
	// save). CommitSettings refreshes the runtime snapshot on success, so the
	// theme read below observes the committed value.
	settings := *m.runtime.Settings()
	settings.ActiveTheme = desired
	if err := m.runtime.CommitSettings(&settings, m.runtime.Machine()); err != nil {
		logging.Warn("failed to save config (auto-theme sync): %v", err)
	}

	ApplyTheme(NewThemeConfig(m.runtime.Settings()))

	return nil
}
