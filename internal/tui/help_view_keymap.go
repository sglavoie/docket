package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for HelpViewModel.
func (m *HelpViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("/", "Search keybindings", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind(`\`, "Search and focus tab", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("backspace", "Clear search filter", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous category tab", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next category tab", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("j", "Scroll down", "Help", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Scroll up", "Help", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Help", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Help", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("g", "Go to top", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("G", "Go to bottom", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+u", "Page up", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+d", "Page down", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("?", "Close help", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("esc", "Clear search / close", "Help", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Help",
		Entries:  entries,
	}
}
