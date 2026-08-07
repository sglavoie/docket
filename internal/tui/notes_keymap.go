package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotesViewModel.
func (m *NotesViewModel) buildKeyMap() *keymap.KeyMap {
	hasRow := func() bool { return m.currentRow() != nil }
	canRename := func() bool {
		row := m.currentRow()
		return row != nil && row.Note != nil && row.Note.ParseError == ""
	}
	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation",
			keymap.WithGate(hasRow),
			keymap.WithFooter(20, "navigate", false),
			keymap.WithFooterKey("j/k")),
		keymap.Bind("k", "Move cursor up", "Navigation"),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden()),
		keymap.Bind("enter", "Open in external editor", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(1, "open/edit", false),
			keymap.WithFooterKey("Enter")),
		keymap.Bind("n", "Create note after choosing location", "Actions",
			keymap.WithFooter(2, "new", false)),

		keymap.Bind("a", "Create note after choosing location", "Actions", keymap.AsHidden()),

		keymap.Bind("v", "Choose Notes scope", "View", keymap.WithFooter(3, "scope", false)),
		keymap.Bind("s", "Search note titles and bodies", "View", keymap.WithFooter(4, "search", false)),

		keymap.Bind("r", "Rename note", "Actions",
			keymap.WithGate(canRename), keymap.WithFooter(5, "rename", false)),

		keymap.Bind("m", "Move note to another location", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(6, "move", false)),

		keymap.Bind("x", "Delete note", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(7, "delete", false)),
		keymap.Bind("u", "Undo workspace change", "Actions", keymap.WithFooter(8, "undo", false)),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions",
			keymap.WithFooter(9, "redo", false), keymap.WithFooterKey("Ctrl+R")),
	}
	return &keymap.KeyMap{Entries: entries}
}

// HandledKeys returns all keys NotesViewModel's Update method can process.
func (m *NotesViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
