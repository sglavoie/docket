package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for CrossProjectLinkerModel.
func (m *CrossProjectLinkerModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("/", "Search", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("enter", "Select", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("g", "Jump to top", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Cross-Project Link",
		Entries:  entries,
	}
}
