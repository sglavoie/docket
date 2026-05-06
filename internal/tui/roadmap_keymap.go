package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for RoadmapModel.
func (m *RoadmapModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Tabs
	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
	)

	// Horizontal scroll
	entries = append(entries,
		keymap.Bind("H", "Scroll left", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("L", "Scroll right", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// Vertical navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("g", "Jump to top", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+d", "Half-page down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+u", "Half-page up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// View controls
	entries = append(entries,
		keymap.Bind("z", "Cycle zoom level", "View", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("t", "Cycle zoom forward", "View", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("T", "Cycle zoom backward", "View", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("space", "Collapse/expand row", "View", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("c", "Toggle critical path highlight", "View", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("s", "Toggle scope (global/project)", "View",
			func() (tea.Model, tea.Cmd) { return m, nil },
			keymap.WithGate(func() bool { return m.canToggle }),
		),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Go to task in project tree", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("o", "Open task detail", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("r", "Refresh roadmap data", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("?", "Toggle help", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Roadmap",
		Entries:  entries,
	}
}
