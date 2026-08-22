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

		// The manager reaches both ends through the strategy's core bindings, whose
		// descriptors answer only to the Outliner, so these two unranked rows are
		// the only place its own `?` help can name them. Each is spelled as the
		// whole path — the chord and its alternate key — rather than as the one
		// physical key the row carries.
		keymap.Bind("home", "Jump to first note", "Navigation",
			keymap.WithGate(hasRow), keymap.WithHelpKey("gg/Home")),
		keymap.Bind("G", "Jump to last note", "Navigation",
			keymap.WithGate(hasRow), keymap.WithHelpKey("G/End")),
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

		// The copies rank last so the footer sheds them before any key that
		// changes a note. They are the widest pair here and the cheapest to
		// rediscover from ? help, and nothing else in this table moves.
		keymap.Bind("y", "Copy note title", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(10, "copy title", false)),
		keymap.Bind("Y", "Copy note path", "Actions",
			keymap.WithGate(hasRow), keymap.WithFooter(11, "copy path", false)),
	}
	return &keymap.KeyMap{Entries: entries}
}

// HandledKeys returns all keys NotesViewModel's Update method can process.
func (m *NotesViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
