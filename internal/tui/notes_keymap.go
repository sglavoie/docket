package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotesViewModel.
func (m *NotesViewModel) buildKeyMap() *keymap.KeyMap {
	if m.combined {
		entries := []keymap.Entry{
			keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
			keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
			keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
			keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
			keymap.Bind("enter", "Open in external editor", "Actions", func() (tea.Model, tea.Cmd) { return m, m.editCurrentNote() }),
			keymap.Bind("n", "Create note after choosing location", "Actions", func() (tea.Model, tea.Cmd) {
				m.beginDestinationPicker()
				return m, nil
			}),
			keymap.Bind("r", "Rename note", "Actions", func() (tea.Model, tea.Cmd) {
				m.startRename()
				return m, nil
			}),
			keymap.Bind("x", "Delete note", "Actions", func() (tea.Model, tea.Cmd) { return m, m.deleteCurrentNote() }),
			keymap.Bind("u", "Undo workspace change", "Actions", func() (tea.Model, tea.Cmd) { return m, m.undoDelete() }),
			keymap.Bind("ctrl+r", "Redo workspace change", "Actions", func() (tea.Model, tea.Cmd) { return m, m.redoNoteMutation() }),
		}
		return &keymap.KeyMap{ViewName: "Notes", Entries: entries}
	}
	var entries []keymap.Entry
	undoDescription := "Undo project change"
	redoDescription := "Redo project change"
	if m.isGlobal {
		undoDescription = "Undo last note change"
		redoDescription = "Redo last note change"
	}

	// Navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+u", "Page up", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+d", "Page down", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("tab", "Toggle details pane focus", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("Z", "Toggle fullscreen pane", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	entries = append(entries,
		keymap.Bind("home", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) {
			m.cursor = 0
			return m, nil
		}),
		keymap.Bind("X", "Deselect all (multi-select)", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.multiSelectMode = false
			m.selectedNotes = make(map[string]bool)
			m.clearStatus()
			return m, nil
		}, keymap.WithGate(func() bool { return m.multiSelectMode })),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "View note (glow)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.viewCurrentNote()
		}),
		keymap.Bind("e", "Edit note", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.editCurrentNote()
		}),
		keymap.Bind("ctrl+o", "Open note reference copy", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openCurrentNoteReference()
		}),
		keymap.Bind("n", "Create new note", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.createNote()
		}),
		keymap.Bind("d", "Delete note (confirm)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				m.confirm.Show("bulk-delete", "Delete notes?")
				return m, nil
			}
			return m, m.deleteCurrentNote()
		}),
		keymap.Bind("x", "Delete note (confirm)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				m.confirm.Show("bulk-delete", "Delete notes?")
				return m, nil
			}
			return m, m.deleteCurrentNote()
		}, keymap.AsHidden()),
		keymap.Bind("u", undoDescription, "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.undoDelete()
		}),
		keymap.Bind("ctrl+r", redoDescription, "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.redoNoteMutation()
		}),
		keymap.Bind("r", "Rename note (inline)", "Actions", func() (tea.Model, tea.Cmd) {
			m.startRename()
			return m, nil
		}),
		keymap.Bind("ctrl+y", "Copy title and body", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.copyNoteTitleAndBody()
		}),
	)

	// Filters
	entries = append(entries,
		keymap.Bind("s", "Cycle sort mode", "Filters", func() (tea.Model, tea.Cmd) {
			m.cycleSortMode()
			return m, nil
		}),
	)

	// Multi-select
	entries = append(entries,
		keymap.Bind("V", "Toggle multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleNoteMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle note selection (multi-select)", "Multi-select", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("a", "Select all notes (multi-select)", "Multi-select", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "Notes",
		Entries:  entries,
	}
}

// HandledKeys returns all keys NotesViewModel's Update method can process.
func (m *NotesViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
