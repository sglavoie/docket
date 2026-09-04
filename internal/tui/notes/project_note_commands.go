package notes

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectNoteService is the narrow typed application boundary used by project
// note views and overlays. Production supplies *app.Service; tests can inject a
// deterministic fake without exposing canonical note paths.
type ProjectNoteService interface {
	NotesAtLocations([]app.NoteLocation) ([]app.NoteCollectionResult, error)
	ApplyNoteBatch(app.NoteBatchRequest) (app.NoteBatchResult, error)
}

// NoteMutationRef is an immutable, path-free snapshot of a note selected for a
// pending operation. Keeping the note by value prevents later view refreshes
// from changing the revision or content that the eventual command commits.
type NoteMutationRef struct {
	Location app.NoteLocation
	Note     models.Note
}

func NewNoteMutationRef(location app.NoteLocation, note *models.Note) NoteMutationRef {
	copyNote := CloneNoteForCommand(note)
	if copyNote == nil {
		return NoteMutationRef{Location: location}
	}
	return NoteMutationRef{Location: location, Note: *copyNote}
}

func (r NoteMutationRef) Valid() bool {
	return r.Location.Scope != "" && r.Note.Filename != ""
}

func (r NoteMutationRef) noteCopy() *models.Note {
	return CloneNoteForCommand(&r.Note)
}

func CloneNoteForCommand(note *models.Note) *models.Note {
	if note == nil {
		return nil
	}
	clone := *note
	return &clone
}

func CloneNoteBatchRequest(req app.NoteBatchRequest) app.NoteBatchRequest {
	copyReq := req
	copyReq.Operations = make([]app.NoteMutationRequest, len(req.Operations))
	for i, operation := range req.Operations {
		copyReq.Operations[i] = operation
		copyReq.Operations[i].Note = CloneNoteForCommand(operation.Note)
	}
	return copyReq
}

// NoteMutationCmd freezes the complete request before starting an asynchronous
// project or global note commit.
func NoteMutationCmd(service ProjectNoteService, location app.NoteLocation, req app.NoteBatchRequest, success string) tea.Cmd {
	request := CloneNoteBatchRequest(req)
	return func() tea.Msg {
		if service == nil {
			return ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: fmt.Errorf("note service is unavailable")}
		}
		result, err := service.ApplyNoteBatch(request)
		if err != nil {
			return ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: err}
		}
		return ProjectNoteMutationResultMsg{
			Location:       location,
			ProjectID:      location.ProjectID,
			Result:         result,
			SuccessMessage: success,
		}
	}
}

func noteLedgerRestoreCmd(ledger *storage.WorkspaceLedger, location app.NoteLocation, scope storage.LedgerScope, redo bool) tea.Cmd {
	return func() tea.Msg {
		if ledger == nil {
			return ProjectNoteLedgerErrorMsg{Location: location, ProjectID: location.ProjectID, Err: fmt.Errorf("workspace undo ledger is unavailable")}
		}
		restore := ledger.UndoWithEntry
		if redo {
			restore = ledger.RedoWithEntry
		}
		entry, action, err := restore(scope)
		if err != nil {
			return ProjectNoteLedgerErrorMsg{Location: location, ProjectID: location.ProjectID, Err: err}
		}
		return ProjectNoteLedgerResultMsg{Location: location, ProjectID: location.ProjectID, Entry: entry, Action: action, Redo: redo}
	}
}
