package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for ScoreProfileManagerModel.
func (m *ScoreProfileManagerModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("G", "Jump to last", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("a", "Add preset profile", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("d", "Delete profile", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("enter", "Assign to project", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("space", "Assign to project", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("u", "Unassign from project", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// Goto group: g prefix (gg = top)
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, nil },
		GoToProject: nil,
	})...)

	return &keymap.KeyMap{
		ViewName: "Score Profile Manager",
		Entries:  entries,
	}
}
