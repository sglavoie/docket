package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for BlockersModel.
func (m *BlockersModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("G", "Jump to last", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("tab", "Switch tabs", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// Goto group: g prefix (gg = top)
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, nil },
		GoToProject: nil,
	})...)

	// Actions
	entries = append(entries,
		keymap.Bind("a", "Add blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("n", "Add blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("e", "Edit blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("E", "Edit in external editor", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("R", "Toggle resolved", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("d", "Delete blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("x", "Delete blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("enter", "Expand blocker", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		// Add/edit mode keys
		keymap.Bind("ctrl+s", "Save (edit/add mode)", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+e", "Open in external editor (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Blockers",
		Entries:  entries,
	}
}
