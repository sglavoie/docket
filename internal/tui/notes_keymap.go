package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for NotesViewModel.
func (m *NotesViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

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

	// g prefix: gg = top
	entries = append(entries, keymap.Prefix("g", "Go…", "Navigation", []keymap.Entry{
		keymap.Bind("g", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("X", "Deselect all (multi-select)", "Multi-select", func() (tea.Model, tea.Cmd) { return m, nil },
			keymap.WithGate(func() bool { return m.multiSelectMode })),
	}))

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "View note (glow)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.viewCurrentNote()
		}),
		keymap.Bind("e", "Edit note", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.editCurrentNote()
		}),
		keymap.Bind("ctrl+o", "Open in notes editor", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openInNotesEditor()
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
		keymap.Bind("u", "Undo last delete", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.undoDelete()
		}),
		keymap.Bind("L", "Link note to task", "Actions", func() (tea.Model, tea.Cmd) {
			if m.isGlobal {
				return m, nil
			}
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkLinkNotes()
			}
			return m, m.linkCurrentNote()
		}),
		keymap.Bind("m", "Move note to project", "Actions", func() (tea.Model, tea.Cmd) {
			if m.isGlobal {
				return m, m.moveNoteToProject()
			}
			return m, nil
		}),
		keymap.Bind("p", "Toggle pin", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.togglePin()
		}),
		keymap.Bind("r", "Rename note (inline)", "Actions", func() (tea.Model, tea.Cmd) {
			m.startRename()
			return m, nil
		}),
		keymap.Bind("ctrl+y", "Copy title and body", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.copyNoteTitleAndBody()
		}),
	)

	// Tags
	entries = append(entries,
		keymap.Bind("t", "Edit note tags", "Tags", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				return m, m.bulkTagNotes()
			}
			return m, m.showTagSelector()
		}),
		keymap.Bind("T", "Cycle tag filter", "Tags", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("C", "Clear tag filters", "Tags", func() (tea.Model, tea.Cmd) {
			if len(m.filterTags) > 0 {
				m.filterTags = make(map[string]bool)
				m.loadNotes()
				m.setStatus("Filter cleared")
			}
			return m, nil
		}),
	)

	// Filters
	entries = append(entries,
		keymap.Bind("/", "Search notes", "Filters", func() (tea.Model, tea.Cmd) { return m, nil }),
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
