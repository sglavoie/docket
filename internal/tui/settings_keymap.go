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
			keymap.Bind("Enter", "Save settings", "Actions", noop),
			keymap.Bind("ctrl+s", "Save settings", "Actions", noop, keymap.AsHidden()),
			keymap.Bind("/", "Search fields", "Search", noop),
			keymap.Bind("ctrl+d", "Reset field to default", "Actions", noop),
			keymap.Bind("ctrl+o", "Load palette preset", "Palette", noop),
			keymap.Bind("ctrl+r", "Reset palette to built-in", "Palette", noop),
			keymap.Bind("?", "Help", "General", noop),
			keymap.Bind("Esc", "Cancel / back", "Navigation", noop),
		},
	}
}
