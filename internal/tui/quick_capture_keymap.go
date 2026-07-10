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

	// Syntax-guide scroll bindings, active only while the guide is expanded (and
	// autocomplete is not shadowing it). Gated so they are documented in help and
	// counted in HandledKeys without appearing as ghost bindings when the guide
	// is closed.
	helpScrollActive := func() bool { return m.helpExpanded && !m.isAutocompleteActive() }
	entries = append(entries,
		keymap.Bind("ctrl+k", "Scroll syntax up", "Syntax", func() (tea.Model, tea.Cmd) {
			m.syntaxScrollLine(-1)
			return m, nil
		}, keymap.WithGate(helpScrollActive)),
		keymap.Bind("ctrl+j", "Scroll syntax down", "Syntax", func() (tea.Model, tea.Cmd) {
			m.syntaxScrollLine(1)
			return m, nil
		}, keymap.WithGate(helpScrollActive)),
		keymap.Bind("pgup", "Syntax page up", "Syntax", func() (tea.Model, tea.Cmd) {
			m.syntaxScrollPage(-1)
			return m, nil
		}, keymap.WithGate(helpScrollActive)),
		keymap.Bind("pgdown", "Syntax page down", "Syntax", func() (tea.Model, tea.Cmd) {
			m.syntaxScrollPage(1)
			return m, nil
		}, keymap.WithGate(helpScrollActive)),
	)

	return &keymap.KeyMap{
		ViewName: "QuickCapture",
		Entries:  entries,
	}
}
