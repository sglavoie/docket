package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

func (m *NotesViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	if m.combined {
		return m.updateCombined(msg)
	}
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.PasteMsg:
		if m.renaming {
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()

		// Handle discard confirmation for rename mode
		if m.confirmDiscard {
			switch key {
			case "y", "Y":
				m.confirmDiscard = false
				m.renaming = false
				m.setStatus("Rename canceled")
				return m, nil
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		// Handle rename input mode
		if m.renaming {
			return m.handleRenameKey(msg)
		}

		// Handle delete confirmation mode
		if m.confirm.IsActive() {
			action := m.confirm.Action()
			if confirmed, handled := m.handleConfirmKey(key); handled {
				if confirmed {
					switch action {
					case "delete":
						if m.pendingDeleteIdx < len(m.notes) {
							return m, m.performDelete()
						}
						m.setStatus("Delete failed: note no longer exists")
					case "bulk-delete":
						return m, m.performBulkDelete()
					}
				}
				return m, nil
			}
		}

		m.clearStatus()

		// Tab toggles focus between list and details pane
		if key == "tab" {
			m.detailsPane.Focused = !m.detailsPane.Focused
			if m.detailsPane.Focused {
				m.detailsPane.ResetScroll()
			}
			return m, nil
		}

		// Z toggles fullscreen (no-op during rename/delete/multiselect)
		if key == "Z" && !m.renaming && !m.confirm.IsActive() && !m.multiSelectMode {
			m.fullscreenPane = !m.fullscreenPane
			m.resetScrollOffset()
			return m, nil
		}

		// Esc exits fullscreen before anything else
		if key == "esc" && m.fullscreenPane {
			m.fullscreenPane = false
			m.resetScrollOffset()
			return m, nil
		}

		// Multi-select exit takes priority over details pane routing
		if m.multiSelectMode && key == "esc" {
			m.multiSelectMode = false
			m.selectedNotes = make(map[string]bool)
			m.clearStatus()
			return m, nil
		}

		// Route keys when details pane is focused
		if m.detailsPane.Focused {
			return m.handleDetailsPaneKey(key)
		}

		switch key {
		case "shift+left", "shift+right":
			if handleTabCycleKey(key, &m.tabs, m.loadNotes) {
				return m, nil
			}
		case "esc":
			if m.isGlobal {
				return m, func() tea.Msg { return CloseGlobalNotesMsg{} }
			}
			return m, func() tea.Msg { return CloseNotesMsg{} }
		case "j", "down", "k", "up", "G", "ctrl+u", "pgup", "ctrl+d", "pgdown":
			prev := m.cursor
			_, navCmd := handleListNavigation(key, &m.cursor, len(m.notes), m.height, &m.cursorFlash)
			if m.cursor != prev {
				m.detailsPane.ResetScroll()
			}
			if navCmd != nil {
				return m, navCmd
			}
		case "home":
			m.cursor = 0
			m.detailsPane.ResetScroll()
			m.resetScrollOffset()
			return m, nil
		case "V":
			m.toggleNoteMultiSelectMode()
			return m, nil
		case "space":
			if !m.multiSelectMode {
				m.toggleNoteMultiSelectMode()
			}
			m.toggleNoteSelection()
			// Move cursor down after toggle.
			if m.cursor < len(m.notes)-1 {
				m.cursor++
				m.detailsPane.ResetScroll()
			}
			return m, nil
		case "a":
			if m.multiSelectMode {
				m.selectAllNotes()
				return m, nil
			}
		case "enter":
			return m, m.viewCurrentNote()
		case "e":
			return m, m.editCurrentNote()
		case "ctrl+o":
			return m, m.openCurrentNoteReference()
		case "n":
			return m, m.createNote()
		case "x", "d":
			if m.multiSelectMode && m.selectedNoteCount() > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d notes?", m.selectedNoteCount()))
				return m, nil
			}
			return m, m.deleteCurrentNote()
		case "u":
			return m, m.undoDelete()
		case "ctrl+r":
			return m, m.redoNoteMutation()
		case "s":
			m.cycleSortMode()
			return m, nil
		case "y":
			return m, m.copyNoteTitle()
		case "Y":
			return m, m.copyNoteBody()
		case "ctrl+y":
			return m, m.copyNoteTitleAndBody()
		case "r":
			m.startRename()
			return m, nil
		case "?":
			sourceView := ViewNotes
			viewName := "Notes"
			if m.isGlobal {
				sourceView = ViewGlobalNotes
				viewName = "Global Notes"
			}
			return m, helpCmd(sourceView, viewName, m.GetHelpItems())
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case NotesRefreshMsg:
		m.tasksLoaded = false
		m.loadNotes()
	}
	return m, nil
}

//nolint:gocyclo // The combined Notes tool keeps its keyboard-state transitions in one dispatcher.
func (m *NotesViewModel) updateCombined(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.renaming {
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if m.confirmDiscard {
			switch key {
			case "y", "Y":
				m.confirmDiscard = false
				m.renaming = false
				m.setStatus("Rename canceled")
			case "n", "N", "esc":
				m.confirmDiscard = false
			}
			return m, nil
		}
		if m.renaming {
			return m.handleRenameKey(msg)
		}
		if m.destinationPicker {
			switch key {
			case "j", "down":
				if m.destinationCursor < len(m.destinations)-1 {
					m.destinationCursor++
				}
			case "k", "up":
				if m.destinationCursor > 0 {
					m.destinationCursor--
				}
			case "enter":
				location := m.destinations[m.destinationCursor]
				m.destinationPicker = false
				return m, m.createNoteAt(location)
			case "esc":
				m.destinationPicker = false
				m.setStatus("Note creation canceled")
			}
			return m, nil
		}
		if m.confirm.IsActive() {
			if confirmed, handled := m.handleConfirmKey(key); handled {
				if confirmed && m.pendingDeleteIdx < len(m.notes) {
					return m, m.performDelete()
				}
				return m, nil
			}
		}

		m.clearStatus()
		switch key {
		case "j", "down", "k", "up", "G", "ctrl+u", "pgup", "ctrl+d", "pgdown":
			_, cmd := handleListNavigation(key, &m.cursor, len(m.notes), m.height, &m.cursorFlash)
			return m, cmd
		case "home":
			m.cursor = 0
		case "enter":
			return m, m.editCurrentNote()
		case "n":
			m.beginDestinationPicker()
		case "r":
			m.startRename()
		case "x":
			return m, m.deleteCurrentNote()
		case "u":
			return m, m.undoDelete()
		case "ctrl+r":
			return m, m.redoNoteMutation()
		case "?":
			return m, helpCmd(ViewNotes, "Notes", m.GetHelpItems())
		case "esc":
			if m.preferred.Scope == app.NoteScopeProject {
				return m, func() tea.Msg { return CloseNotesMsg{} }
			}
			return m, func() tea.Msg { return CloseGlobalNotesMsg{} }
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case NotesRefreshMsg:
		m.loadNotes()
	}
	return m, nil
}

// ShouldCaptureEsc returns true while Notes owns an internal interaction layer.
func (m *NotesViewModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.detailsPane.Focused || m.multiSelectMode || m.renaming || m.fullscreenPane || m.confirmDiscard || m.destinationPicker
}

// HasTextInput returns false since search is now a separate view.
func (m *NotesViewModel) HasTextInput() bool {
	return m.renaming
}

func (m *NotesViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	// Scroll keys handled by shared state
	if m.detailsPane.HandleScrollKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.detailsPane.Focused = false
	case "e":
		return m, m.editCurrentNote()
	case "ctrl+o":
		return m, m.openCurrentNoteReference()
	case "enter":
		return m, m.viewCurrentNote()
	}
	return m, nil
}

func (m *NotesViewModel) handleRenameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newTitle := strings.TrimSpace(m.renameInput.Value())
		if newTitle == "" {
			m.setStatus("Rename canceled: empty title")
			m.renaming = false
			return m, nil
		}
		if m.cursor >= len(m.notes) {
			m.renaming = false
			return m, nil
		}
		note := m.notes[m.cursor]
		oldTitle := note.Title
		if newTitle == oldTitle {
			m.renaming = false
			m.setStatus("Rename canceled: same title")
			return m, nil
		}
		location := m.noteLocationFor(note)
		m.renaming = false
		return m, noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{{Kind: app.NoteRename, Location: location, Note: note, NewTitle: newTitle}},
			UndoAction:  "Rename Note",
			UndoContext: oldTitle,
		}, fmt.Sprintf("Renamed: %s → %s", oldTitle, newTitle), false)
	case "esc":
		if m.cursor < len(m.notes) && strings.TrimSpace(m.renameInput.Value()) != m.notes[m.cursor].Title {
			m.confirmDiscard = true
			return m, nil
		}
		m.renaming = false
		m.setStatus("Rename canceled")
		return m, nil
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// startRename begins inline rename mode for the current note.
func (m *NotesViewModel) startRename() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	if m.notes[m.cursor].ParseError != "" {
		m.setErrorStatus("cannot rename — note has a parse error; press Enter to repair it")
		return
	}
	ti := textinput.New()
	ti.SetValue(m.notes[m.cursor].Title)
	ti.CharLimit = 100
	ti.SetWidth(50)
	ti.Focus()
	applyThemedTextInput(&ti)
	m.renameInput = ti
	m.renaming = true
}
