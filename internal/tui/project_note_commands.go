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

// noteMutationRef is an immutable, path-free snapshot of a note selected for a
// pending operation. Keeping the note by value prevents later view mutations
// from changing the revision or content that the eventual command will commit.
type noteMutationRef struct {
	Location app.NoteLocation
	Note     models.Note
}

func newNoteMutationRef(location app.NoteLocation, note *models.Note) noteMutationRef {
	copyNote := cloneNoteForCommand(note)
	if copyNote == nil {
		return noteMutationRef{Location: location}
	}
	return noteMutationRef{Location: location, Note: *copyNote}
}

func newNoteMutationRefs(location app.NoteLocation, notes []*models.Note) []noteMutationRef {
	refs := make([]noteMutationRef, 0, len(notes))
	for _, note := range notes {
		if note != nil {
			refs = append(refs, newNoteMutationRef(location, note))
		}
	}
	return refs
}

func (r noteMutationRef) noteCopy() *models.Note {
	return cloneNoteForCommand(&r.Note)
}

func noteRefsToCopies(refs []noteMutationRef) []*models.Note {
	notes := make([]*models.Note, 0, len(refs))
	for _, ref := range refs {
		notes = append(notes, ref.noteCopy())
	}
	return notes
}

func cloneNoteForCommand(note *models.Note) *models.Note {
	if note == nil {
		return nil
	}
	clone := *note
	clone.Tags = append([]string(nil), note.Tags...)
	clone.LinkedTasks = append([]string(nil), note.LinkedTasks...)
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

// projectNoteMutationCmd freezes the complete request before starting the
// asynchronous commit. A command never retains a live Bubble Tea note pointer.
func projectNoteMutationCmd(service projectNoteService, projectID string, req app.NoteBatchRequest, success string, clearSelection bool) tea.Cmd {
	return noteMutationCmd(service, app.ProjectNoteLocation(projectID), req, success, clearSelection)
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
