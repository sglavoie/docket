package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for QuickCaptureModel.
func (m *QuickCaptureModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("enter", "Create task", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.createTask(false, false)
		}),
		keymap.Bind("ctrl+d", "Create task and edit description", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.createTask(true, false)
		}),
		keymap.Bind("ctrl+o", "Create task and open detail view", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.createTask(false, true)
		}),
		keymap.Bind("ctrl+v", "Paste clipboard text into input", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "QuickCapture",
		Entries:  entries,
	}
}
