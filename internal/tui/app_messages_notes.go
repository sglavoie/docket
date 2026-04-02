package tui

import "github.com/sglavoie/tt/internal/models"

// =============================================================================
// Notes Messages (Show/Close Pattern)
// =============================================================================

// ShowNotesMsg opens the notes view.
type ShowNotesMsg struct {
	Project *models.Project
}

// CloseNotesMsg closes the notes view.
type CloseNotesMsg struct{}

// NotesRefreshMsg refreshes the notes list.
type NotesRefreshMsg struct{}

// ShowGlobalNotesMsg opens the global notes view.
type ShowGlobalNotesMsg struct{}

// CloseGlobalNotesMsg closes the global notes view.
type CloseGlobalNotesMsg struct{}

// MoveNoteToProjectMsg moves a note from global notes to a project.
type MoveNoteToProjectMsg struct {
	Note    *models.Note
	Project *models.Project
}

// ShowNoteCreatorMsg opens a quick note creation prompt.
type ShowNoteCreatorMsg struct {
	Project *models.Project
}

// ShowNoteTaskLinkerMsg opens the task picker to link a note to a task.
type ShowNoteTaskLinkerMsg struct {
	Project *models.Project
	Note    *models.Note
}

// BulkNoteTagMsg opens the tag selector for multiple notes.
type BulkNoteTagMsg struct {
	Notes    []*models.Note
	Project  *models.Project
	NotesDir string
	IsGlobal bool
}

// BulkNoteTagDoneMsg returns the result from bulk note tagging.
type BulkNoteTagDoneMsg struct {
	Notes        []*models.Note
	Project      *models.Project
	NotesDir     string
	IsGlobal     bool
	SelectedTags []string
	Canceled     bool
}

// BulkNoteLinkMsg opens the task linker for multiple notes.
type BulkNoteLinkMsg struct {
	Notes   []*models.Note
	Project *models.Project
}

// BulkNoteLinkDoneMsg returns the result from bulk note linking.
type BulkNoteLinkDoneMsg struct {
	Notes    []*models.Note
	Project  *models.Project
	TaskID   string
	Canceled bool
}

// CloseNoteTaskLinkerMsg closes the note task linker.
type CloseNoteTaskLinkerMsg struct{}

// NoteTaskLinkedMsg signals a note was linked to a task.
type NoteTaskLinkedMsg struct {
	TaskID string // Last toggled task ID, used for bulk linking
}
