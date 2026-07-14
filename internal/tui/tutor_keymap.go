package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TutorModel.
func (m *TutorModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("/", "Search", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("tab", "Switch pane (list/content)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous category", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next category", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("up", "Move cursor up / scroll content up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up / scroll content up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down / scroll content down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("j", "Move cursor down / scroll content down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("g", "Scroll to top (content pane)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("G", "Scroll to bottom (content pane)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+u", "Page up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+d", "Page down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("enter", "Switch to content pane", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Reference",
		Entries:  entries,
	}
}
