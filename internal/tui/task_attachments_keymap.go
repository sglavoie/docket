package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskAttachmentsModel.
func (m *TaskAttachmentsModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.addMode }

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

		keymap.Bind("G", "Jump to last attachment", "Navigation", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	entries = append(entries, keymap.Bind("home", "Jump to first attachment", "Navigation", keymap.WithGate(func() bool {
		return !inInputMode()
	}),
	))

	// Open/view actions
	entries = append(entries,
		keymap.Bind("a", "Add attachment", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("o", "Open (system default)", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("enter", "Open (system default)", "Actions", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("e", "Open in editor", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("ctrl+e", "Open original path (files)", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("ctrl+o", "Reveal original file in file manager", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("O", "Open attachments directory", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	// Mutation actions
	entries = append(entries,
		keymap.Bind("d", "Delete (removes file)", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("x", "Delete (removes file)", "Actions", keymap.AsHidden(), keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("u", "Unlink (keep file)", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("y", "Copy path to clipboard", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("Y", "Copy path with line (code refs)", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("J", "Reorder attachment down", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),

		keymap.Bind("K", "Reorder attachment up", "Actions", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	// Help
	entries = append(entries,
		keymap.Bind("?", "Toggle help", "Navigation", keymap.WithGate(func() bool {
			return !inInputMode()
		}),
		),
	)

	return &keymap.KeyMap{
		Entries: entries,
	}
}
