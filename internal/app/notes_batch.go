package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

const maxNoteCollisionAttempts = 1000

type preparedNoteOperation struct {
	kind             NoteMutationKind
	source           NoteLocation
	destination      NoteLocation
	note             *models.Note
	previousFilename string
	baseFilename     string
	collisionIndex   int
}

type preparedNoteArtifact struct {
	change      storage.ArtifactChange
	operation   int
	destination bool
}

// ApplyNoteBatch validates every request against one workspace snapshot and
// commits all resulting artifacts atomically. Existing sources always carry the
// caller's observed raw revision. New destinations always require absence.
// Filename collisions are discovered only by the locked commit boundary; a
// collision advances that destination to -2, -3, ... and retries the whole batch.
func (s *Service) ApplyNoteBatch(req NoteBatchRequest) (NoteBatchResult, error) {
	if len(req.Operations) == 0 {
		return NoteBatchResult{}, invalidInput("note batch", "at least one operation is required")
	}
	committer, ok := s.repo.(ArtifactCommitter)
	if !ok {
		return NoteBatchResult{}, fmt.Errorf("app: repository does not support artifact-only note commits")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return NoteBatchResult{}, err
	}
	prepared, err := s.prepareNoteOperations(snap, req.Operations)
	if err != nil {
		return NoteBatchResult{}, err
	}

	projectIDs := noteProjectIDs(prepared)
	for attempt := 0; attempt < maxNoteCollisionAttempts; attempt++ {
		artifacts, results, retryBuild, buildErr := s.buildNoteArtifacts(snap, prepared)
		if buildErr != nil {
			return NoteBatchResult{}, buildErr
		}
		if retryBuild {
			continue
		}
		var preconditions []storage.ArtifactPrecondition
		if len(projectIDs) > 0 {
			projectPrecondition, preconditionErr := s.notes.ProjectLocationPrecondition(noteProjects(snap, projectIDs))
			if preconditionErr != nil {
				return NoteBatchResult{}, preconditionErr
			}
			if projectPrecondition != nil {
				preconditions = append(preconditions, *projectPrecondition)
			}
		}
		action, context := noteUndoMetadata(req, results)
		transactionID, commitErr := committer.CommitArtifactTransaction(storage.ArtifactCommitRequest{
			Artifacts:     artifactChanges(artifacts),
			Preconditions: preconditions,
			ProjectIDs:    projectIDs,
			UndoAction:    action,
			UndoContext:   context,
		})
		if commitErr == nil {
			for i := range results {
				results[i].TransactionID = transactionID
			}
			return NoteBatchResult{Results: results, TransactionID: transactionID}, nil
		}
		var conflict *storage.ArtifactConflictError
		if !errors.As(commitErr, &conflict) || conflict.Expected != "" || !advanceCollision(prepared, artifacts, conflict.Path) {
			return NoteBatchResult{}, commitErr
		}
	}
	return NoteBatchResult{}, fmt.Errorf("app: note filename collision limit reached after %d attempts", maxNoteCollisionAttempts)
}

func (s *Service) prepareNoteOperations(snap Snapshot, requests []NoteMutationRequest) ([]preparedNoteOperation, error) {
	prepared := make([]preparedNoteOperation, len(requests))
	for i, req := range requests {
		if req.Note == nil {
			return nil, invalidInput("note", "note is required")
		}
		note := cloneNote(req.Note)
		if err := validateNoteMutation(req.Kind, note); err != nil {
			return nil, err
		}
		op := preparedNoteOperation{kind: req.Kind, source: req.Location, destination: req.Location, note: note, previousFilename: note.Filename, collisionIndex: 1}
		switch req.Kind {
		case NoteCreate:
			if _, err := resolveNoteLocation(snap, req.Location); err != nil {
				return nil, err
			}
			if note.RawRevision != "" {
				return nil, invalidInput("note revision", "a new note cannot carry an observed source revision")
			}
			if note.CreatedAt.IsZero() {
				note.CreatedAt = s.clock.Now()
			}
			if note.ModifiedAt.IsZero() {
				note.ModifiedAt = s.clock.Now()
			}
			if strings.TrimSpace(note.Filename) == "" {
				note.Filename = models.GenerateNoteFilename(note.Title, note.CreatedAt)
			}
			op.previousFilename = ""
			op.baseFilename = note.Filename
		case NoteUpdate:
			if _, err := resolveNoteLocation(snap, req.Location); err != nil {
				return nil, err
			}
			if err := requireObservedNote(note); err != nil {
				return nil, err
			}
			if note.ModifiedAt.IsZero() {
				note.ModifiedAt = s.clock.Now()
			}
			op.baseFilename = note.Filename
		case NoteRename:
			if _, err := resolveNoteLocation(snap, req.Location); err != nil {
				return nil, err
			}
			if err := requireObservedNote(note); err != nil {
				return nil, err
			}
			newTitle := strings.TrimSpace(req.NewTitle)
			if newTitle == "" {
				return nil, invalidInput("note title", "new title is required")
			}
			note.Title = newTitle
			note.ModifiedAt = s.clock.Now()
			op.baseFilename = models.GenerateNoteFilename(newTitle, note.CreatedAt)
			note.Filename = op.baseFilename
		case NoteMove:
			if _, err := resolveNoteLocation(snap, req.Location); err != nil {
				return nil, err
			}
			if _, err := resolveNoteLocation(snap, req.Destination); err != nil {
				return nil, err
			}
			if req.Location == req.Destination {
				return nil, invalidInput("note destination", "source and destination must differ")
			}
			if err := requireObservedNote(note); err != nil {
				return nil, err
			}
			op.destination = req.Destination
			op.baseFilename = note.Filename
		case NoteDelete:
			if _, err := resolveNoteLocation(snap, req.Location); err != nil {
				return nil, err
			}
			if err := requireObservedNote(note); err != nil {
				return nil, err
			}
			op.baseFilename = note.Filename
		default:
			return nil, invalidInput("note operation", fmt.Sprintf("unsupported kind %q", req.Kind))
		}
		prepared[i] = op
	}
	return prepared, nil
}

func validateNoteMutation(kind NoteMutationKind, note *models.Note) error {
	// Delete consumes only the observed filename and raw revision. Requiring
	// parsed metadata here would make a malformed note impossible to remove.
	if kind == NoteDelete {
		return nil
	}
	if note.ParseError != "" {
		return invalidInput("note", "a note with invalid frontmatter cannot be mutated")
	}
	if strings.TrimSpace(note.Title) == "" && kind != NoteRename {
		return invalidInput("note", "title is required")
	}
	return nil
}

func (s *Service) buildNoteArtifacts(snap Snapshot, operations []preparedNoteOperation) ([]preparedNoteArtifact, []NoteResult, bool, error) {
	artifacts := make([]preparedNoteArtifact, 0, len(operations)*2)
	results := make([]NoteResult, len(operations))
	for i := range operations {
		op := &operations[i]
		sourceProject, err := resolveNoteLocation(snap, op.source)
		if err != nil {
			return nil, nil, false, err
		}
		destinationProject := sourceProject
		if op.destination != op.source {
			destinationProject, err = resolveNoteLocation(snap, op.destination)
			if err != nil {
				return nil, nil, false, err
			}
		}
		note := cloneNote(op.note)
		note.Filename = collisionFilename(op.baseFilename, op.collisionIndex)

		result := NoteResult{
			Kind:      op.kind,
			Location:  op.destination,
			ProjectID: op.destination.ProjectID,
			Filename:  note.Filename,
			Title:     note.Title,
		}
		switch op.kind {
		case NoteCreate:
			change, encErr := s.notes.EncodeNoteSave(op.destination, destinationProject, note)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			change.ExpectedRevision = stringPointer("")
			note.RawRevision = storage.RawArtifactRevision(change.Data)
			artifacts = append(artifacts, preparedNoteArtifact{change: change, operation: i, destination: true})
		case NoteUpdate:
			change, encErr := s.notes.EncodeNoteSave(op.source, sourceProject, note)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			change.ExpectedRevision = stringPointer(op.note.RawRevision)
			note.RawRevision = storage.RawArtifactRevision(change.Data)
			artifacts = append(artifacts, preparedNoteArtifact{change: change, operation: i})
		case NoteRename, NoteMove:
			write, encErr := s.notes.EncodeNoteSave(op.destination, destinationProject, note)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			write.ExpectedRevision = stringPointer("")
			remove, encErr := s.notes.EncodeNoteDelete(op.source, sourceProject, op.previousFilename)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			remove.ExpectedRevision = stringPointer(op.note.RawRevision)
			note.RawRevision = storage.RawArtifactRevision(write.Data)
			artifacts = append(artifacts,
				preparedNoteArtifact{change: write, operation: i, destination: true},
				preparedNoteArtifact{change: remove, operation: i},
			)
		case NoteDelete:
			remove, encErr := s.notes.EncodeNoteDelete(op.source, sourceProject, op.previousFilename)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			remove.ExpectedRevision = stringPointer(op.note.RawRevision)
			result.Location = op.source
			result.ProjectID = op.source.ProjectID
			result.Filename = op.previousFilename
			artifacts = append(artifacts, preparedNoteArtifact{change: remove, operation: i})
		}
		results[i] = result
	}

	seen := make(map[string]int, len(artifacts))
	for i := range artifacts {
		path := filepath.Clean(artifacts[i].change.Path)
		if previous, exists := seen[path]; exists {
			candidate := i
			if !artifacts[candidate].destination {
				candidate = previous
			}
			if artifacts[candidate].destination && bumpCollision(&operations[artifacts[candidate].operation]) {
				return nil, nil, true, nil
			}
			return nil, nil, false, invalidInput("note batch", "the same source is mutated more than once")
		}
		seen[path] = i
	}
	return artifacts, results, false, nil
}
