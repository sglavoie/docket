package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// toggleNoteMultiSelectMode enters or exits multi-select mode.
func (m *NotesViewModel) toggleNoteMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.selectedNotes = make(map[string]bool)
		m.statusMessage = ""
	} else {
		m.statusMessage = "Multi-select: space=toggle, a=all, gX=clear"
	}
}

// toggleNoteSelection toggles selection of the current note.
func (m *NotesViewModel) toggleNoteSelection() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	note := m.notes[m.cursor]
	if m.selectedNotes[note.Filename] {
		delete(m.selectedNotes, note.Filename)
	} else {
		m.selectedNotes[note.Filename] = true
	}
	m.updateNoteSelectionStatus()
}

// selectAllNotes selects all visible notes.
func (m *NotesViewModel) selectAllNotes() {
	for _, note := range m.notes {
		m.selectedNotes[note.Filename] = true
	}
	m.updateNoteSelectionStatus()
}

// deselectAllNotes clears all note selections.
func (m *NotesViewModel) deselectAllNotes() {
	m.selectedNotes = make(map[string]bool)
	m.updateNoteSelectionStatus()
}

// getSelectedNotesList returns a slice of all selected notes.
func (m *NotesViewModel) getSelectedNotesList() []*models.Note {
	var notes []*models.Note
	for _, note := range m.notes {
		if m.selectedNotes[note.Filename] {
			notes = append(notes, note)
		}
	}
	return notes
}

// isNoteSelected returns whether a note is currently selected.
func (m *NotesViewModel) isNoteSelected(filename string) bool {
	return m.selectedNotes[filename]
}

// selectedNoteCount returns the number of selected notes.
func (m *NotesViewModel) selectedNoteCount() int {
	return len(m.selectedNotes)
}

// updateNoteSelectionStatus updates the status message with selection count.
func (m *NotesViewModel) updateNoteSelectionStatus() {
	count := len(m.selectedNotes)
	if count == 0 {
		m.statusMessage = "Multi-select: space=toggle, a=all, gX=clear"
	} else {
		m.statusMessage = fmt.Sprintf("%d selected (space=toggle, d=delete, t=tag, l=link, gX=clear)", count)
	}
}
