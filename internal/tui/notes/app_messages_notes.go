package notes

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/route"
)

// =============================================================================
// Notes Messages (Show/Close Pattern)
// =============================================================================

// ShowNotesManagerMsg opens the workspace-wide Notes manager. Scope selects its
// initial All/Global/project view. Focus is optional; its zero value leaves the
// manager's default selection unchanged.
type ShowNotesManagerMsg struct {
	Scope      route.NotesScope
	Focus      NoteRowIdentity
	MoveSource NoteMutationRef
}

// CloseNotesManagerMsg closes the Notes manager's single secondary-tool route.
type CloseNotesManagerMsg struct{ route.PopViewMsg }

// ProjectNoteMutationResultMsg reports one committed note transaction. Location
// may be either a project collection or the workspace-global collection. The command that emits
// it owns only copied request data; AppModel.Update reloads from committed state.
type ProjectNoteMutationResultMsg struct {
	Location       app.NoteLocation
	ProjectID      string
	Result         app.NoteBatchResult
	SuccessMessage string
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
