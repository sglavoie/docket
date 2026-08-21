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
		keymap.Bind("a", "Create note after choosing location", "Actions",
			keymap.WithFooter(2, "new", false)),

		keymap.Bind("e", "Rename note", "Actions",
			keymap.WithGate(canRename), keymap.WithFooter(3, "rename", false)),

		keymap.Bind("%", "Move note to another location", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(4, "move", false)),

		keymap.Bind("/", "Search note titles and bodies", "View", keymap.WithFooter(5, "search", false)),
		keymap.Bind("p", "Choose Notes scope", "View", keymap.WithFooter(6, "scope", false)),

		keymap.Bind("space", "Open note actions", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(7, "actions", false),
			keymap.WithFooterKey("Space")),
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
