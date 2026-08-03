package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for HelpViewModel.
func (m *HelpViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("/", "Search keybindings", "Help"),
		keymap.Bind(`\`, "Search and focus tab", "Help"),
		keymap.Bind("backspace", "Clear search filter", "Help"),
		keymap.Bind("shift+left", "Previous category tab", "Help"),
		keymap.Bind("shift+right", "Next category tab", "Help"),
		keymap.Bind("j", "Scroll down", "Help", keymap.AsHidden()),
		keymap.Bind("k", "Scroll up", "Help", keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Help", keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Help", keymap.AsHidden()),
		keymap.Bind("g", "Go to top", "Help"),
		keymap.Bind("G", "Go to bottom", "Help"),
		keymap.Bind("ctrl+u", "Page up", "Help"),
		keymap.Bind("ctrl+d", "Page down", "Help"),
		keymap.Bind("?", "Close help", "Help"),
		keymap.Bind("esc", "Clear search / close", "Help"),
	)

	return &keymap.KeyMap{
		Entries: entries,
	}
}
