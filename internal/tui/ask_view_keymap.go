package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for AskViewModel.
// The ask view manages its own state machine (input/loading/result) so the
// Dispatcher is not used for dispatch; this keymap is used only for HandledKeys().
func (m *AskViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Input state key
	entries = append(entries,
		keymap.Bind("ctrl+s", "Submit query", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("ctrl+t", "Cycle AI context (full/redacted/off)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	// Result state keys
	entries = append(entries,
		keymap.Bind("a", "Review and apply structured plan (result)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("n", "New query (result)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "AI Assistant",
		Entries:  entries,
	}
}
