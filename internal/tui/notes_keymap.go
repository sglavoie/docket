package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotesViewModel.
func (m *NotesViewModel) buildKeyMap() *keymap.KeyMap {
	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("enter", "Open in external editor", "Actions", func() (tea.Model, tea.Cmd) { return m, m.editCurrentNote() }),
		keymap.Bind("n", "Create note after choosing location", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginDestinationPicker()
		}),
		keymap.Bind("a", "Create note after choosing location", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginDestinationPicker()
		}),
		keymap.Bind("v", "Choose Notes scope", "View", func() (tea.Model, tea.Cmd) { return m, m.beginScopePicker() }),
		keymap.Bind("s", "Search note titles and bodies", "View", func() (tea.Model, tea.Cmd) {
			return m, m.beginSearch()
		}),
		keymap.Bind("r", "Rename note", "Actions", func() (tea.Model, tea.Cmd) {
			m.startRename()
			return m, nil
		}),
		keymap.Bind("m", "Move note to another location", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginMovePicker()
		}),
		keymap.Bind("x", "Delete note", "Actions", func() (tea.Model, tea.Cmd) { return m, m.deleteCurrentNote() }),
		keymap.Bind("u", "Undo workspace change", "Actions", func() (tea.Model, tea.Cmd) { return m, m.undoDelete() }),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions", func() (tea.Model, tea.Cmd) { return m, m.redoNoteMutation() }),
	}
	return &keymap.KeyMap{ViewName: "Notes", Entries: entries}
}

// HandledKeys returns all keys NotesViewModel's Update method can process.
func (m *NotesViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
