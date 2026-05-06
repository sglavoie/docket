package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for FilterBuilderModel.
// The filter builder uses a custom focus/navigation system, so the Dispatcher
// is not used for dispatch; this keymap is used only for HandledKeys().
func (m *FilterBuilderModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Tab navigation
	entries = append(entries,
		keymap.Bind("shift+left", "Previous tab (Criteria/Options)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("shift+right", "Next tab (Criteria/Options)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	// Field navigation
	entries = append(entries,
		keymap.Bind("tab", "Navigate to next field", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("shift+tab", "Navigate to previous field", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("enter", "Next field", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("left", "Cycle field/op/value left", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("right", "Cycle field/op/value right", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("h", "Cycle field/op/value left (vim-style)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("l", "Cycle field/op/value right (vim-style)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("k", "Move cursor up (vim-style)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("j", "Move cursor down (vim-style)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("space", "Toggle (logic/sort/completed)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("+", "Add criterion", "Criteria", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("-", "Remove criterion", "Criteria", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("ctrl+s", "Save smart list", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Filter Builder",
		Entries:  entries,
	}
}
