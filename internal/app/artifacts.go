package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectArtifactCodec encodes a project's note and blocker mutations into the
// storage.ArtifactChange values that ride a ChangeSet, keeping the application
// layer free of filesystem-path and serialization concerns. The codec performs no
// I/O — it computes a target path and the bytes to write (or a delete) — so one
// implementation serves both an in-memory repository and the real filesystem
// commit service; only the Repository that applies the ChangeSet differs.
type ProjectArtifactCodec interface {
	// EncodeNoteSave encodes a note write under the project's notes directory.
	EncodeNoteSave(project *models.Project, note *models.Note) (storage.ArtifactChange, error)
	// EncodeNoteDelete encodes the removal of a note file by filename.
	EncodeNoteDelete(project *models.Project, filename string) (storage.ArtifactChange, error)
	// EncodeBlockers encodes a whole-list rewrite of the project's blockers.md.
	EncodeBlockers(project *models.Project, blockers []*models.Blocker) (storage.ArtifactChange, error)
}

// ProjectArtifactReader reads a project's notes and blockers. Notes and blockers
// live in per-project files outside the task snapshot, so reads go through this
// port rather than the WorkspaceSnapshot. The storage-backed reader reads the
// canonical files from disk; an in-memory reader (tests) reads a map, so every
// query is exercised against both a filesystem and an in-memory implementation.
type ProjectArtifactReader interface {
	Notes(project *models.Project) ([]*models.Note, error)
	Blockers(project *models.Project) ([]*models.Blocker, error)
}

// storageArtifactCodec is the default, I/O-free codec: it derives the canonical
// on-disk paths and renders the canonical byte form using the storage package's
// note and blocker serializers.
type storageArtifactCodec struct{}

func (storageArtifactCodec) EncodeNoteSave(project *models.Project, note *models.Note) (storage.ArtifactChange, error) {
	if note == nil {
		return storage.ArtifactChange{}, invalidInput("note", "note is required")
	}
	path, err := storage.NoteFilePathIn(storage.NotesDir(project), note.Filename)
	if err != nil {
		return storage.ArtifactChange{}, invalidInput("note", err.Error())
	}
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	return storage.ArtifactChange{Path: path, Data: []byte(content)}, nil
}

func (storageArtifactCodec) EncodeNoteDelete(project *models.Project, filename string) (storage.ArtifactChange, error) {
	path, err := storage.NoteFilePathIn(storage.NotesDir(project), filename)
	if err != nil {
		return storage.ArtifactChange{}, invalidInput("note", err.Error())
	}
	return storage.ArtifactChange{Path: path, Delete: true}, nil
}

func (storageArtifactCodec) EncodeBlockers(project *models.Project, blockers []*models.Blocker) (storage.ArtifactChange, error) {
	content := storage.RenderBlockersContent(blockers)
	return storage.ArtifactChange{Path: storage.BlockersFilePath(project), Data: []byte(content)}, nil
}

// storageArtifactReader is the default filesystem reader for notes and blockers.
type storageArtifactReader struct{}

func (storageArtifactReader) Notes(project *models.Project) ([]*models.Note, error) {
	return storage.ListNotes(project)
}

func (storageArtifactReader) Blockers(project *models.Project) ([]*models.Blocker, error) {
	return storage.LoadBlockers(project)
}

// NoteResult is the presentation-neutral outcome of a note mutation.
type NoteResult struct {
	// ProjectID owns the note.
	ProjectID string
	// Filename is the note's on-disk filename.
	Filename string
	// Title is the note's title (empty for a delete).
	Title string
	// Revision is the owning project's revision after the commit.
	Revision string
}

// BlockersResult is the presentation-neutral outcome of a blocker-list mutation.
type BlockersResult struct {
	// ProjectID owns the blockers.
	ProjectID string
	// Count is the total number of blockers persisted.
	Count int
	// ActiveCount is the number of unresolved blockers.
	ActiveCount int
	// Revision is the owning project's revision after the commit.
	Revision string
}

// SaveNote persists a note in a project as a transaction artifact committed
// atomically with the project's tasks.md. The note is written under the project's
// notes directory (a durable synced artifact). A save with an existing filename
// overwrites in place — callers needing collision avoidance mint a fresh filename
// via models.GenerateNoteFilename before calling. Missing timestamps and filename
// are defaulted so a caller may pass only a title and body.
func (s *Service) SaveNote(projectID string, note *models.Note) (NoteResult, error) {
	if note == nil {
		return NoteResult{}, invalidInput("note", "note is required")
	}
	if strings.TrimSpace(note.Title) == "" {
		return NoteResult{}, invalidInput("note", "title is required")
	}
	_, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return NoteResult{}, err
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
	change, err := s.codec.EncodeNoteSave(project, note)
	if err != nil {
		return NoteResult{}, err
	}
	rev, err := s.commitArtifact(ArtifactNotes, project, revision, change, "Save Note", note.Title)
	if err != nil {
		return NoteResult{}, err
	}
	return NoteResult{ProjectID: project.ID, Filename: note.Filename, Title: note.Title, Revision: rev}, nil
}

// DeleteNote removes a note file from a project, committing the removal atomically
// with the project. Deleting a note that does not exist surfaces the underlying
// storage error only for a filesystem repository; the request itself is validated
// before any commit.
func (s *Service) DeleteNote(projectID, filename string) (NoteResult, error) {
	if strings.TrimSpace(filename) == "" {
		return NoteResult{}, invalidInput("note", "filename is required")
	}
	_, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return NoteResult{}, err
	}
	change, err := s.codec.EncodeNoteDelete(project, filename)
	if err != nil {
		return NoteResult{}, err
	}
	rev, err := s.commitArtifact(ArtifactNotes, project, revision, change, "Delete Note", filename)
	if err != nil {
		return NoteResult{}, err
	}
	return NoteResult{ProjectID: project.ID, Filename: filename, Revision: rev}, nil
}

// SetBlockers replaces a project's whole blocker list, committing the rewritten
// blockers.md atomically with the project. Each blocker is validated (non-empty
// title) and defaulted (a fresh ID and creation time when unset) before the
// commit, so a rejected request never persists a partial list.
func (s *Service) SetBlockers(projectID string, blockers []*models.Blocker) (BlockersResult, error) {
	_, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return BlockersResult{}, err
	}
	active := 0
	for _, b := range blockers {
		if b == nil {
			return BlockersResult{}, invalidInput("blocker", "blocker is required")
		}
		if strings.TrimSpace(b.Title) == "" {
			return BlockersResult{}, invalidInput("blocker", "title is required")
		}
		if strings.TrimSpace(b.ID) == "" {
			b.ID = s.ids.NewID()
		}
		if b.CreatedAt.IsZero() {
			b.CreatedAt = s.clock.Now()
		}
		if !b.IsResolved() {
			active++
		}
	}
	change, err := s.codec.EncodeBlockers(project, blockers)
	if err != nil {
		return BlockersResult{}, err
	}
	rev, err := s.commitArtifact(ArtifactBlockers, project, revision, change, "Update Blockers", project.Name)
	if err != nil {
		return BlockersResult{}, err
	}
	return BlockersResult{ProjectID: project.ID, Count: len(blockers), ActiveCount: active, Revision: rev}, nil
}

// commitArtifact commits one project write plus a single durable artifact change as
// one transaction, returning the project's new revision. It refuses a machine-local
// artifact kind: machine-local UI state must never be committed to the durable
// workspace, so routing it here is a programming error caught before any commit.
func (s *Service) commitArtifact(kind ArtifactKind, project *models.Project, expectedRevision string, change storage.ArtifactChange, undoAction, undoContext string) (string, error) {
	if !kind.IsDurable() {
		return "", invalidInput("artifact", fmt.Sprintf("%s is not durable synced state and cannot be committed to the workspace", kind))
	}
	applyPostMutationHooks(project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          project,
		ExpectedRevision: expectedRevision,
		Artifacts:        []storage.ArtifactChange{change},
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return "", err
	}
	return revisionAt(results, 0), nil
}
