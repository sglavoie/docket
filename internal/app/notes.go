package app

import (
	"github.com/sglavoie/tt/internal/models"
)

// This file holds the note vocabulary the rest of the workspace speaks: the
// path-free location identity, the request and result shapes, and the
// single-operation Service methods. The storage-backed port lives in
// notes_port.go, the atomic batch pipeline in notes_batch.go, and its shared
// helpers in notes_support.go.

// NoteScope identifies which durable workspace collection owns a note.
type NoteScope string

const (
	NoteScopeProject NoteScope = "project"
	NoteScopeGlobal  NoteScope = "global"
)

// NoteLocation is the path-free identity of a note collection. ProjectID is
// required only for project scope. CLI and TUI callers retain this value instead
// of canonical directories or mutable project pointers.
type NoteLocation struct {
	Scope     NoteScope
	ProjectID string
}

// ProjectNoteLocation returns the project-scoped note collection identity.
func ProjectNoteLocation(projectID string) NoteLocation {
	return NoteLocation{Scope: NoteScopeProject, ProjectID: projectID}
}

// GlobalNoteLocation returns the workspace-global note collection identity.
func GlobalNoteLocation() NoteLocation { return NoteLocation{Scope: NoteScopeGlobal} }

// NoteMutationKind describes one operation inside an atomic note batch.
type NoteMutationKind string

const (
	NoteCreate NoteMutationKind = "create"
	NoteUpdate NoteMutationKind = "update"
	NoteRename NoteMutationKind = "rename"
	NoteMove   NoteMutationKind = "move"
	NoteDelete NoteMutationKind = "delete"
)

// NoteCreateRequest creates Note at Location. The destination is always
// revision-checked as expected absent.
type NoteCreateRequest struct {
	Location NoteLocation
	Note     *models.Note
}

// NoteUpdateRequest replaces an existing note. Note.RawRevision must be the
// revision observed when the source was loaded.
type NoteUpdateRequest struct {
	Location NoteLocation
	Note     *models.Note
}

// NoteRenameRequest atomically creates the renamed destination and deletes the
// observed source. NewTitle determines both the persisted title and filename.
type NoteRenameRequest struct {
	Location NoteLocation
	Note     *models.Note
	NewTitle string
}

// NoteMoveRequest atomically moves an observed note between explicit workspace
// locations without exposing either location's canonical directory.
type NoteMoveRequest struct {
	Source      NoteLocation
	Destination NoteLocation
	Note        *models.Note
}

// NoteDeleteRequest deletes an observed note. Note.RawRevision is mandatory.
type NoteDeleteRequest struct {
	Location NoteLocation
	Note     *models.Note
}

// NoteMutationRequest is the batch representation of create, update, rename,
// move, and delete. The single-operation request types above are converted into
// this form by their corresponding Service methods.
type NoteMutationRequest struct {
	Kind        NoteMutationKind
	Location    NoteLocation
	Destination NoteLocation
	Note        *models.Note
	NewTitle    string
}

// NoteBatchRequest applies every operation as one journal and ledger entry.
// UndoAction and UndoContext are optional presentation metadata.
type NoteBatchRequest struct {
	Operations  []NoteMutationRequest
	UndoAction  string
	UndoContext string
}

// NoteResult is the path-free presentation-neutral outcome of one note mutation.
type NoteResult struct {
	Kind          NoteMutationKind
	Location      NoteLocation
	ProjectID     string // compatibility convenience; mirrors Location.ProjectID
	Filename      string
	Title         string
	TransactionID string
}

// NoteBatchResult reports every operation in request order and the shared
// transaction ID proving the batch committed atomically.
type NoteBatchResult struct {
	Results       []NoteResult
	TransactionID string
}

// CreateNote commits one project or global note create.
func (s *Service) CreateNote(req NoteCreateRequest) (NoteResult, error) {
	return s.singleNoteMutation(NoteBatchRequest{Operations: []NoteMutationRequest{{Kind: NoteCreate, Location: req.Location, Note: req.Note}}, UndoAction: "Create Note"})
}

// UpdateNote commits one revision-checked project or global note replacement.
func (s *Service) UpdateNote(req NoteUpdateRequest) (NoteResult, error) {
	return s.singleNoteMutation(NoteBatchRequest{Operations: []NoteMutationRequest{{Kind: NoteUpdate, Location: req.Location, Note: req.Note}}, UndoAction: "Update Note"})
}

// RenameNote atomically renames one project or global note.
func (s *Service) RenameNote(req NoteRenameRequest) (NoteResult, error) {
	return s.singleNoteMutation(NoteBatchRequest{Operations: []NoteMutationRequest{{Kind: NoteRename, Location: req.Location, Note: req.Note, NewTitle: req.NewTitle}}, UndoAction: "Rename Note"})
}

// MoveNote atomically moves one note between project/global locations.
func (s *Service) MoveNote(req NoteMoveRequest) (NoteResult, error) {
	return s.singleNoteMutation(NoteBatchRequest{Operations: []NoteMutationRequest{{Kind: NoteMove, Location: req.Source, Destination: req.Destination, Note: req.Note}}, UndoAction: "Move Note"})
}

// RemoveNote commits one revision-checked project or global note delete.
func (s *Service) RemoveNote(req NoteDeleteRequest) (NoteResult, error) {
	return s.singleNoteMutation(NoteBatchRequest{Operations: []NoteMutationRequest{{Kind: NoteDelete, Location: req.Location, Note: req.Note}}, UndoAction: "Delete Note"})
}

func (s *Service) singleNoteMutation(req NoteBatchRequest) (NoteResult, error) {
	result, err := s.ApplyNoteBatch(req)
	if err != nil {
		return NoteResult{}, err
	}
	single := result.Results[0]
	if single.TransactionID == "" {
		single.TransactionID = result.TransactionID
	}
	return single, nil
}
