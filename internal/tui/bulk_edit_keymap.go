package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for BulkEditModel.
func (m *BulkEditModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("tab", "Next field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("down", "Next field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("j", "Next field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("up", "Previous field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Previous field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		// Field interaction
		keymap.Bind("enter", "Confirm", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("space", "Toggle selection", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("l", "Cycle option right", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("right", "Cycle option right", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("h", "Cycle option left", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("left", "Cycle option left", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		// Tag operations
		keymap.Bind("a", "Add tag", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("r", "Remove tag", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Bulk Edit",
		Entries:  entries,
	}
}
