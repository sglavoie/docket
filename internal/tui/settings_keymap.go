package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for SettingsModel.
// The entries are used only for the help system; actual routing goes through handleKeyPress.
func (m *SettingsModel) buildKeyMap() *keymap.KeyMap {
	noop := func() (tea.Model, tea.Cmd) { return m, nil }

	return &keymap.KeyMap{
		ViewName: "Settings",
		Entries: []keymap.Entry{
			keymap.Bind("shift+←/→", "Switch tab", "Navigation", noop),
			keymap.Bind("Tab", "Next field", "Navigation", noop),
			keymap.Bind("shift+Tab", "Previous field", "Navigation", noop),
			keymap.Bind("Space", "Toggle / cycle", "Actions", noop),
			keymap.Bind("Enter", "Activate / next field", "Actions", noop),
			keymap.Bind("ctrl+s", "Save settings", "Actions", noop),
			keymap.Bind("/", "Search fields", "Search", noop),
			keymap.Bind("?", "Help", "General", noop),
			keymap.Bind("Esc", "Cancel / back", "Navigation", noop),
		},
	}
}
