package help

import (
	"github.com/sglavoie/docket/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for HelpViewModel.
func (m *HelpViewModel) buildKeyMap() *keymap.KeyMap {
	if m.actionSearch() && !m.tabSearchMode {
		return m.actionKeyMap()
	}
	browsing := func() bool { return !m.searchMode && !m.tabSearchMode }
	searchLabel, searchHint := "Search keybindings", "filter"
	if m.actionContext != nil {
		searchLabel, searchHint = "Search actions and shortcuts", "find action"
	}
	hasFilter := func() bool { return m.searchQuery != "" }
	entries := []keymap.Entry{
		keymap.Bind("tab", "Show all matches", "Help",
			keymap.WithGate(func() bool { return !m.tabSearchMode && m.otherCategoryMatches > 0 }),
			keymap.WithFooter(1, "show all matches", true), keymap.WithFooterKey("Tab")),
		keymap.Bind("/", searchLabel, "Help",
			keymap.WithGate(browsing), keymap.WithFooter(20, searchHint, false)),
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
		keymap.Bind("g", "Go to top (gg)", "Help"),
		keymap.Bind("home", "Go to top", "Help", keymap.AsHidden()),
		keymap.Bind("G", "Go to bottom", "Help"),
		keymap.Bind("end", "Go to bottom", "Help", keymap.AsHidden()),
		keymap.Bind("ctrl+u", "Page up", "Help"),
		keymap.Bind("ctrl+d", "Page down", "Help"),
		keymap.Bind("pgup", "Page up", "Help", keymap.AsHidden()),
		keymap.Bind("pgdown", "Page down", "Help", keymap.AsHidden()),
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

func (m *HelpViewModel) actionKeyMap() *keymap.KeyMap {
	browsing := func() bool { return !m.searchMode }
	runnable := func() bool {
		item, ok := m.selectedActionItem()
		return ok && item.Action != nil && item.DisabledReason == ""
	}
	return &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("enter", "Run selected action", "Help", keymap.WithGate(runnable),
			keymap.WithFooter(1, "run", true), keymap.WithFooterKey("Enter")),
		keymap.Bind("enter", "Keep help open", "Help", keymap.WithGate(func() bool { return !runnable() }), keymap.AsHidden()),
		keymap.Bind("esc", "Finish typing", "Help", keymap.WithGate(func() bool { return m.searchMode }),
			keymap.WithFooter(2, "finish typing", true), keymap.WithFooterKey("Esc")),
		keymap.Bind("?", "Close help", "Help", keymap.WithGate(browsing), keymap.WithFooter(2, "close", true)),
		keymap.Bind("up", "Choose previous result", "Help", keymap.WithFooter(3, "choose", true), keymap.WithFooterKey("↑/↓")),
		keymap.Bind("down", "Choose next result", "Help"),
		keymap.Bind("k", "Choose previous result", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("j", "Choose next result", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("tab", "Show all matches", "Help", keymap.WithGate(func() bool { return m.otherCategoryMatches > 0 }),
			keymap.WithFooter(4, "show all matches", true), keymap.WithFooterKey("Tab")),
		keymap.Bind("/", "Search actions", "Help", keymap.WithGate(browsing), keymap.WithFooter(5, "filter", false)),
		keymap.Bind("backspace", "Clear search filter", "Help", keymap.WithGate(browsing),
			keymap.WithFooter(6, "clear", false), keymap.WithFooterKey("Bksp")),
		keymap.Bind("esc", "Clear search filter", "Help", keymap.WithGate(browsing),
			keymap.WithFooter(7, "clear filter", false), keymap.WithFooterKey("Esc")),
		keymap.Bind("left", "Previous category tab", "Help", keymap.WithGate(browsing),
			keymap.WithFooter(8, "tabs", false), keymap.WithFooterKey("←/→")),
		keymap.Bind("right", "Next category tab", "Help", keymap.WithGate(browsing)),
		keymap.Bind("h", "Previous category tab", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("l", "Next category tab", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind(`\`, "Search and focus tab", "Help", keymap.WithGate(browsing), keymap.WithFooter(9, "find tab", false)),
		keymap.Bind("g", "Go to first result (gg)", "Help", keymap.WithGate(browsing)),
		keymap.Bind("home", "Go to first result", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("G", "Go to last result", "Help", keymap.WithGate(browsing)),
		keymap.Bind("end", "Go to last result", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("ctrl+u", "Previous page of results", "Help", keymap.WithGate(browsing)),
		keymap.Bind("ctrl+d", "Next page of results", "Help", keymap.WithGate(browsing)),
		keymap.Bind("pgup", "Previous page of results", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
		keymap.Bind("pgdown", "Next page of results", "Help", keymap.WithGate(browsing), keymap.AsHidden()),
	}}
}
