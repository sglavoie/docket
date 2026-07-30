package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// projectNoteService is the narrow typed application boundary used by project
// note views and overlays. Production supplies *app.Service; tests can inject a
// deterministic fake without exposing canonical note paths.
type projectNoteService interface {
	NotesAt(app.NoteLocation) ([]*models.Note, error)
	ApplyNoteBatch(app.NoteBatchRequest) (app.NoteBatchResult, error)
}

func cloneNoteForCommand(note *models.Note) *models.Note {
	if note == nil {
		return nil
	}
	clone := *note
	return &clone
}

func cloneNoteBatchRequest(req app.NoteBatchRequest) app.NoteBatchRequest {
	copyReq := req
	copyReq.Operations = make([]app.NoteMutationRequest, len(req.Operations))
	for i, operation := range req.Operations {
		copyReq.Operations[i] = operation
		copyReq.Operations[i].Note = cloneNoteForCommand(operation.Note)
	}
	return copyReq
}

// noteMutationCmd freezes the complete request before starting an asynchronous
// project or global note commit.
func noteMutationCmd(service projectNoteService, location app.NoteLocation, req app.NoteBatchRequest, success string, clearSelection bool) tea.Cmd {
	request := cloneNoteBatchRequest(req)
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
			ClearSelection: clearSelection,
		}
	}
}

func noteLedgerRestoreCmd(ledger *storage.WorkspaceLedger, location app.NoteLocation, scope storage.LedgerScope, redo bool) tea.Cmd {
	return func() tea.Msg {
		if ledger == nil {
			return ProjectNoteLedgerErrorMsg{Location: location, ProjectID: location.ProjectID, Redo: redo, Err: fmt.Errorf("workspace undo ledger is unavailable")}
		}
		restore := ledger.UndoWithEntry
		if redo {
			restore = ledger.RedoWithEntry
		}
		entry, action, err := restore(scope)
		if err != nil {
			return ProjectNoteLedgerErrorMsg{Location: location, ProjectID: location.ProjectID, Redo: redo, Err: err}
		}
		return ProjectNoteLedgerResultMsg{Location: location, ProjectID: location.ProjectID, Entry: entry, Action: action, Redo: redo}
	}
}
