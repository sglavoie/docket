package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for CustomContextEditorModel.
func (m *CustomContextEditorModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("tab", "Toggle name/projects", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("/", "Search projects", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("enter", "Toggle selection", "Selection", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("space", "Toggle selection", "Selection", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("ctrl+s", "Save", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("a", "Select all", "Selection", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("A", "Deselect all", "Selection", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Custom Context Editor",
		Entries:  entries,
	}
}
