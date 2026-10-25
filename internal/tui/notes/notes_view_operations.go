package notes

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/external"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/status"
)

func (m *NotesViewModel) EditCurrentNote() tea.Cmd {
	row := m.CurrentRow()
	if row == nil {
		return nil
	}
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return NoteEditCmd(row.Note, row.Location, m.effectiveDir(row), editor)
}

// NoteEditCmd opens one note in the editor. A malformed note is edited as raw
// file content so its frontmatter can be repaired by hand.
func NoteEditCmd(note *models.Note, location app.NoteLocation, dir, editor string) tea.Cmd {
	if note.ParseError != "" {
		path, pathErr := storage.NoteFilePathIn(dir, note.Filename)
		if pathErr != nil {
			return status.ErrorCmd("invalid note path: %w", pathErr)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return status.ErrorCmd("failed to read malformed note: %w", readErr)
		}
		return startNoteEditorWithContent(editor, location, note, string(raw), false)
	}
	return startNoteEditor(editor, location, note, false)
}

func (m *NotesViewModel) beginLocationPicker(
	purpose NotesLocationPickerPurpose,
	excludedLocation app.NoteLocation,
) tea.Cmd {
	m.moveSource = nil
	m.LocationPicker = newNotesLocationPicker(
		purpose,
		m.Scope,
		excludedLocation,
		m.ownerProjects,
	)
	m.LocationPicker.SetSize(m.Width(), m.Height())
	m.LocationPicker.SetAccentColor(m.AccentColor())
	return m.LocationPicker.Init()
}

func (m *NotesViewModel) BeginDestinationPicker() tea.Cmd {
	return m.beginLocationPicker(NotesLocationPickerCreation, app.NoteLocation{})
}

func (m *NotesViewModel) BeginScopePicker() tea.Cmd {
	return m.beginLocationPicker(NotesLocationPickerScope, app.NoteLocation{})
}

func (m *NotesViewModel) BeginMovePicker() tea.Cmd {
	row := m.CurrentRow()
	if row == nil {
		return nil
	}
	return m.BeginMovePickerForSource(NewNoteMutationRef(row.Location, row.Note))
}

func (m *NotesViewModel) BeginMovePickerForSource(source NoteMutationRef) tea.Cmd {
	if !source.Valid() {
		m.SetErrorStatus("Move failed: note is no longer available")
		return nil
	}
	if source.Note.ParseError != "" {
		m.SetErrorStatus("Cannot move: note has a parse error; press Enter to repair it")
		return nil
	}

	picker := newNotesLocationPicker(
		NotesLocationPickerMove,
		m.Scope,
		source.Location,
		m.ownerProjects,
	)
	if len(picker.SourceItems()) == 0 {
		m.moveSource = nil
		m.LocationPicker = nil
		m.SetStatus("No other note locations are available; note was not moved")
		return nil
	}
	frozen := NewNoteMutationRef(source.Location, source.noteCopy())
	m.moveSource = &frozen
	m.LocationPicker = picker
	m.LocationPicker.SetSize(m.Width(), m.Height())
	m.LocationPicker.SetAccentColor(m.AccentColor())
	return m.LocationPicker.Init()
}

func (m *NotesViewModel) createNoteAt(location app.NoteLocation) tea.Cmd {
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		m.SetErrorStatus(err.Error())
		return nil
	}
	return startNoteEditor(editor, location, models.NewNote("Untitled Note"), true)
}

func (m *NotesViewModel) DeleteCurrentNote() {
	row := m.CurrentRow()
	if row == nil {
		return
	}
	m.Confirm.Show("delete", fmt.Sprintf("Delete '%s'?", row.Note.Title))
	m.PendingDelete = row.Identity()
}

func (m *NotesViewModel) PerformDelete() tea.Cmd {
	row := m.rowByIdentity(m.PendingDelete)
	if row == nil {
		return nil
	}
	location := row.Location
	note := row.Note
	return NoteMutationCmd(m.NoteService, location, app.NoteBatchRequest{
		Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: location, Note: note}},
		UndoAction:  "Delete Note",
		UndoContext: note.Title,
	}, "Deleted: "+note.Title)
}

func (m *NotesViewModel) UndoDelete() tea.Cmd {
	return noteLedgerRestoreCmd(
		m.NoteLedger,
		app.GlobalNoteLocation(),
		m.noteLedgerScope(),
		false,
	)
}

func (m *NotesViewModel) RedoNoteMutation() tea.Cmd {
	return noteLedgerRestoreCmd(
		m.NoteLedger,
		app.GlobalNoteLocation(),
		m.noteLedgerScope(),
		true,
	)
}
