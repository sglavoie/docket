package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotesViewModel.
func (m *NotesViewModel) buildKeyMap() *keymap.KeyMap {
	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation"),
		keymap.Bind("k", "Move cursor up", "Navigation"),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden()),
		keymap.Bind("enter", "Open in external editor", "Actions"),
		keymap.Bind("n", "Create note after choosing location", "Actions"),

		keymap.Bind("a", "Create note after choosing location", "Actions"),

		keymap.Bind("v", "Choose Notes scope", "View"),
		keymap.Bind("s", "Search note titles and bodies", "View"),

		keymap.Bind("r", "Rename note", "Actions"),

		keymap.Bind("m", "Move note to another location", "Actions"),

		keymap.Bind("x", "Delete note", "Actions"),
		keymap.Bind("u", "Undo workspace change", "Actions"),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions"),
	}
	return &keymap.KeyMap{Entries: entries}
}

// HandledKeys returns all keys NotesViewModel's Update method can process.
func (m *NotesViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
