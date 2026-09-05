package help

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for HelpViewModel.
func (m *HelpViewModel) buildKeyMap() *keymap.KeyMap {
	browsing := func() bool { return !m.searchMode && !m.tabSearchMode }
	hasFilter := func() bool { return m.searchQuery != "" }
	entries := []keymap.Entry{
		keymap.Bind("/", "Search keybindings", "Help",
			keymap.WithGate(browsing), keymap.WithFooter(20, "filter", false)),
		keymap.Bind(`\`, "Search and focus tab", "Help",
			keymap.WithGate(browsing), keymap.WithFooter(20, "find tab", false)),
		keymap.Bind("backspace", "Clear search filter", "Help",
			keymap.WithGate(func() bool { return browsing() && hasFilter() }),
			keymap.WithFooter(2, "clear filter", true), keymap.WithFooterKey("Bksp")),
		keymap.Bind("left", "Previous category tab", "Help",
			keymap.WithGate(browsing), keymap.WithFooter(10, "tabs", false),
			keymap.WithFooterKey("←/→ h/l")),
		keymap.Bind("right", "Next category tab", "Help"),
		keymap.Bind("h", "Previous category tab", "Help", keymap.AsHidden()),
		keymap.Bind("l", "Next category tab", "Help", keymap.AsHidden()),
		keymap.Bind("j", "Scroll down", "Help",
			keymap.WithGate(browsing), keymap.WithFooter(10, "scroll", false),
			keymap.WithFooterKey("j/k")),
		keymap.Bind("k", "Scroll up", "Help", keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Help", keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Help", keymap.AsHidden()),
		keymap.Bind("g", "Go to top", "Help"),
		keymap.Bind("G", "Go to bottom", "Help"),
		keymap.Bind("ctrl+u", "Page up", "Help"),
		keymap.Bind("ctrl+d", "Page down", "Help"),
		keymap.Bind("?", "Close help", "Help",
			keymap.WithGate(browsing), keymap.WithFooter(1, "close", true)),
		keymap.Bind("esc", "Clear search / close", "Help",
			keymap.WithGate(func() bool { return browsing() && hasFilter() }),
			keymap.WithFooter(1, "clear filter", true), keymap.WithFooterKey("Esc")),
	}

	return &keymap.KeyMap{
		Entries: entries,
	}
}
