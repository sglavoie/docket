package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for MetricsDashboardModel.
func (m *MetricsDashboardModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("t", "Cycle time range forward", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("T", "Cycle time range backward", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("g", "Toggle global/project scope", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("r", "Refresh metrics", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("j", "Move cursor down (Blocked tab)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down (Blocked tab)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up (Blocked tab)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up (Blocked tab)", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("enter", "Navigate to task (Blocked tab)", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("?", "Toggle help", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Metrics Dashboard",
		Entries:  entries,
	}
}
