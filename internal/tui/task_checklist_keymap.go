package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskChecklistModel.
func (m *TaskChecklistModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.addMode || m.editMode }

	var entries []keymap.Entry

	// Normal mode navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("k", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("G", "Jump to last item", "Navigation", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	entries = append(entries, keymap.Bind("home", "Jump to first item", "Navigation", keymap.WithGate(func() bool {
		return !inInputMode()
	}),
	))

	// Normal mode actions
	entries = append(entries,
		keymap.Bind("a", "Add item", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("n", "Add item", "Actions", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("e", "Edit item", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("E", "Edit in external editor", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("d", "Delete item", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("x", "Delete item", "Actions", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("space", "Toggle done/undone", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("y", "Copy item text", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("Y", "Copy all as Markdown", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("J", "Reorder item down", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("K", "Reorder item up", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	// Enter toggles the selected item in normal mode and saves in add/edit mode.
	// Always included in HandledKeys so esc/save handling stays documented.
	entries = append(entries,
		keymap.Bind("enter", "Toggle done/undone / save (add/edit mode)", "Actions"),

		keymap.Bind("ctrl+e", "Open in external editor (add mode)", "Actions"),
	)

	return &keymap.KeyMap{
		Entries: entries,
	}
}
