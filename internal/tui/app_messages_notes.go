package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// =============================================================================
// Notes Messages (Show/Close Pattern)
// =============================================================================

// ShowNotesMsg opens the notes view.
type ShowNotesMsg struct {
	Project *models.Project
}

// CloseNotesMsg closes the notes view.
type CloseNotesMsg struct{ PopViewMsg }

// NotesRefreshMsg refreshes the notes list.
type NotesRefreshMsg struct{}

// ProjectNoteMutationResultMsg reports one committed note transaction. Location
// may be either a project collection or the workspace-global collection. The command that emits
// it owns only copied request data; AppModel.Update reloads from committed state.
type ProjectNoteMutationResultMsg struct {
	Location       app.NoteLocation
	ProjectID      string
	Result         app.NoteBatchResult
	SuccessMessage string
	ClearSelection bool
}

// ProjectNoteMutationErrorMsg preserves the typed application/storage error
// produced by a rejected note transaction. Update handling surfaces it without
// touching the view's note cache or selection.
type ProjectNoteMutationErrorMsg struct {
	Location  app.NoteLocation
	ProjectID string
	Err       error
}

// ProjectNoteLedgerResultMsg reports a completed workspace-ledger restore for
// a project or global note view. Notes reload only after this reaches Update.
type ProjectNoteLedgerResultMsg struct {
	Location  app.NoteLocation
	ProjectID string
	Entry     storage.LedgerEntry
	Action    string
	Redo      bool
}

// ProjectNoteLedgerErrorMsg reports a rejected project or global undo/redo.
type ProjectNoteLedgerErrorMsg struct {
	Location  app.NoteLocation
	ProjectID string
	Err       error
	Redo      bool
}

// ProjectNoteEditorDoneMsg carries only copied note/editor data back from a
// temporary-file editor process. The canonical note is not opened or mutated.
type ProjectNoteEditorDoneMsg struct {
	Location        app.NoteLocation
	ProjectID       string
	Original        *models.Note
	OriginalContent string
	Modified        string
	Create          bool
	Err             error
}

// ShowGlobalNotesMsg opens the global notes view.
type ShowGlobalNotesMsg struct{}

// CloseGlobalNotesMsg closes the global notes view.
type CloseGlobalNotesMsg struct{ PopToAnchorMsg }

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
	Sources       []noteMutationRef
	AvailableTags []TagInfo
}

// BulkNoteLinkMsg opens the task linker for multiple notes.
type BulkNoteLinkMsg struct {
	Notes   []*models.Note
	Project *models.Project
}

// BulkNoteLinkDoneMsg returns the result from bulk note linking.
type BulkNoteLinkDoneMsg struct {
	Notes       []*models.Note
	ProjectID   string
	LinkChanges map[string]bool
	Canceled    bool
}

// CloseNoteTaskLinkerMsg closes the note task linker.
type CloseNoteTaskLinkerMsg struct{}

// NoteTaskLinkedMsg signals a note was linked to a task.
type NoteTaskLinkedMsg struct {
	TaskID string // Last toggled task ID, used for bulk linking
}
