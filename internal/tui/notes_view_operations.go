package tui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *NotesViewModel) editCurrentNote() tea.Cmd {
	row := m.currentRow()
	if row == nil {
		return nil
	}
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		return errorCmd("%s", err)
	}
	return noteEditCmd(row.Note, row.Location, m.effectiveDir(row), editor)
}

// noteEditCmd opens one note in the editor. A malformed note is edited as raw
// file content so its frontmatter can be repaired by hand.
func noteEditCmd(note *models.Note, location app.NoteLocation, dir, editor string) tea.Cmd {
	if note.ParseError != "" {
		path, pathErr := storage.NoteFilePathIn(dir, note.Filename)
		if pathErr != nil {
			return errorCmd("invalid note path: %w", pathErr)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return errorCmd("failed to read malformed note: %w", readErr)
		}
		return startNoteEditorWithContent(editor, location, note, string(raw), false)
	}
	return startNoteEditor(editor, location, note, false)
}

func (m *NotesViewModel) beginLocationPicker(
	purpose notesLocationPickerPurpose,
	excludedLocation app.NoteLocation,
) tea.Cmd {
	m.moveSource = nil
	m.locationPicker = newNotesLocationPicker(
		purpose,
		m.scope,
		excludedLocation,
		m.activeProjects,
	)
	m.locationPicker.SetSize(m.Width(), m.Height())
	m.locationPicker.SetAccentColor(m.AccentColor())
	return m.locationPicker.Init()
}

func (m *NotesViewModel) beginDestinationPicker() tea.Cmd {
	return m.beginLocationPicker(notesLocationPickerCreation, app.NoteLocation{})
}

func (m *NotesViewModel) beginScopePicker() tea.Cmd {
	return m.beginLocationPicker(notesLocationPickerScope, app.NoteLocation{})
}

func (m *NotesViewModel) beginMovePicker() tea.Cmd {
	row := m.currentRow()
	if row == nil {
		return nil
	}
	return m.beginMovePickerForSource(newNoteMutationRef(row.Location, row.Note))
}

func (m *NotesViewModel) beginMovePickerForSource(source noteMutationRef) tea.Cmd {
	if !source.valid() {
		m.setErrorStatus("Move failed: note is no longer available")
		return nil
	}
	if source.Note.ParseError != "" {
		m.setErrorStatus("cannot move — note has a parse error; press Enter to repair it")
		return nil
	}

	picker := newNotesLocationPicker(
		notesLocationPickerMove,
		m.scope,
		source.Location,
		m.activeProjects,
	)
	if len(picker.SourceItems()) == 0 {
		m.moveSource = nil
		m.locationPicker = nil
		m.setStatus("No other note locations are available; note was not moved")
		return nil
	}
	frozen := newNoteMutationRef(source.Location, source.noteCopy())
	m.moveSource = &frozen
	m.locationPicker = picker
	m.locationPicker.SetSize(m.Width(), m.Height())
	m.locationPicker.SetAccentColor(m.AccentColor())
	return m.locationPicker.Init()
}

func (m *NotesViewModel) createNoteAt(location app.NoteLocation) tea.Cmd {
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		m.setErrorStatus(err.Error())
		return nil
	}
	return startNoteEditor(editor, location, models.NewNote("Untitled Note"), true)
}

func (m *NotesViewModel) deleteCurrentNote() {
	row := m.currentRow()
	if row == nil {
		return
	}
	m.confirm.Show("delete", fmt.Sprintf("Delete '%s'?", row.Note.Title))
	m.pendingDelete = row.Identity()
}

func (m *NotesViewModel) performDelete() tea.Cmd {
	row := m.rowByIdentity(m.pendingDelete)
	if row == nil {
		return nil
	}
	location := row.Location
	note := row.Note
	return noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
		Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: location, Note: note}},
		UndoAction:  "Delete Note",
		UndoContext: note.Title,
	}, "Deleted: "+note.Title)
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	return noteLedgerRestoreCmd(
		m.noteLedger,
		app.GlobalNoteLocation(),
		m.noteLedgerScope(),
		false,
	)
}

func (m *NotesViewModel) redoNoteMutation() tea.Cmd {
	return noteLedgerRestoreCmd(
		m.noteLedger,
		app.GlobalNoteLocation(),
		m.noteLedgerScope(),
		true,
	)
}
