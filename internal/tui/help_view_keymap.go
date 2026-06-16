package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for HelpViewModel.
// The isExplorer flag changes the behavior slightly but both modes handle the same keys.
func (m *HelpViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("/", "Search keybindings", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind(`\`, "Search and focus tab", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("backspace", "Clear search filter", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous category tab", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next category tab", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("j", "Scroll down", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Scroll up", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("g", "Go to top", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("G", "Go to bottom", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+u", "Page up", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+d", "Page down", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("p", "Toggle by prefix/category", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind(`ctrl+\`, "Close explorer", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("?", "Toggle help (no-op in explorer)", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("esc", "Clear search / close", "Explorer", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Keybinding Explorer",
		Entries:  entries,
	}
}
