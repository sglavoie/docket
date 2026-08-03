package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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

// WorkspaceNotePort resolves path-free locations, reads notes, and encodes note
// artifacts. It performs no writes; the application layer sends every encoded
// batch through ArtifactCommitter.
type WorkspaceNotePort interface {
	Notes(location NoteLocation, project *models.Project) ([]*models.Note, error)
	NotesForMutation(location NoteLocation, project *models.Project) ([]*models.Note, error)
	EncodeNoteSave(location NoteLocation, project *models.Project, note *models.Note) (storage.ArtifactChange, error)
	EncodeNoteDelete(location NoteLocation, project *models.Project, filename string) (storage.ArtifactChange, error)
	ProjectLocationPrecondition(projects []*models.Project) (*storage.ArtifactPrecondition, error)
}

type storageWorkspaceNotePort struct{ dataDir string }

// NewStorageWorkspaceNotePort returns the canonical storage-backed note port for
// a workspace. dataDir is required for global-note operations; project locations
// derive their directories from the resolved project snapshot.
func NewStorageWorkspaceNotePort(dataDir string) WorkspaceNotePort {
	return storageWorkspaceNotePort{dataDir: dataDir}
}

func (p storageWorkspaceNotePort) Notes(location NoteLocation, project *models.Project) ([]*models.Note, error) {
	dir, err := p.noteDir(location, project)
	if err != nil {
		return nil, err
	}
	return storage.ListNotesFrom(dir)
}

func (p storageWorkspaceNotePort) NotesForMutation(location NoteLocation, project *models.Project) ([]*models.Note, error) {
	dir, err := p.noteDir(location, project)
	if err != nil {
		return nil, err
	}
	return storage.ListNotesForMutationFrom(dir)
}

func (p storageWorkspaceNotePort) EncodeNoteSave(location NoteLocation, project *models.Project, note *models.Note) (storage.ArtifactChange, error) {
	if note == nil {
		return storage.ArtifactChange{}, invalidInput("note", "note is required")
	}
	dir, err := p.noteDir(location, project)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	path, err := storage.NoteFilePathIn(dir, note.Filename)
	if err != nil {
		return storage.ArtifactChange{}, invalidInput("note", err.Error())
	}
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	return storage.ArtifactChange{Path: path, Data: []byte(content)}, nil
}

func (p storageWorkspaceNotePort) EncodeNoteDelete(location NoteLocation, project *models.Project, filename string) (storage.ArtifactChange, error) {
	dir, err := p.noteDir(location, project)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	path, err := storage.NoteFilePathIn(dir, filename)
	if err != nil {
		return storage.ArtifactChange{}, invalidInput("note", err.Error())
	}
	return storage.ArtifactChange{Path: path, Delete: true}, nil
}

// ProjectLocationConflictError reports that a project resolved from the
// operation's workspace snapshot is no longer active at that same catalog
// location. It unwraps to storage.ErrStaleArtifact so adapters can use their
// existing reload-and-retry handling while callers that need detail can inspect
// the project and paths.
type ProjectLocationConflictError struct {
	ProjectID    string
	ExpectedPath string
	ActualPath   string
	Reason       string
}

func (e *ProjectLocationConflictError) Error() string {
	switch e.Reason {
	case "missing":
		return fmt.Sprintf("app: project %s is no longer present in the workspace catalog: reload and retry", e.ProjectID)
	case "archived":
		return fmt.Sprintf("app: project %s is archived and cannot accept note changes: reload and retry", e.ProjectID)
	default:
		return fmt.Sprintf("app: project %s moved from %s to %s: reload and retry", e.ProjectID, e.ExpectedPath, e.ActualPath)
	}
}

func (e *ProjectLocationConflictError) Unwrap() error { return storage.ErrStaleArtifact }

// ProjectLocationPrecondition verifies that every project captured in a note
// batch snapshot is still active at the same catalog-derived directory, then
// returns the exact workspace.json revision observed by that validation. The
// commit boundary locks and rechecks this read-only guard to close catalog races
// after the comparison.
func (p storageWorkspaceNotePort) ProjectLocationPrecondition(projects []*models.Project) (*storage.ArtifactPrecondition, error) {
	if len(projects) == 0 {
		return nil, nil
	}
	dataDir, err := noteWorkspaceDataDir(p.dataDir, projects)
	if err != nil {
		return nil, err
	}
	catalog, revision, err := config.NewDomainFileStore(dataDir).LoadCatalogChecked()
	if err != nil {
		return nil, fmt.Errorf("load workspace catalog for note mutation: %w", err)
	}
	byID := make(map[string]config.CatalogProject, len(catalog.Projects))
	for _, project := range catalog.Projects {
		byID[project.ID] = project
	}
	for _, snapshotProject := range projects {
		if snapshotProject == nil {
			return nil, invalidInput("note location", "resolved project is required")
		}
		current, ok := byID[snapshotProject.ID]
		if !ok {
			return nil, &ProjectLocationConflictError{ProjectID: snapshotProject.ID, ExpectedPath: filepath.Clean(snapshotProject.Path), Reason: "missing"}
		}
		actualPath := filepath.Clean(current.CatalogPath(dataDir))
		expectedPath := filepath.Clean(snapshotProject.Path)
		if current.Archived {
			return nil, &ProjectLocationConflictError{ProjectID: snapshotProject.ID, ExpectedPath: expectedPath, ActualPath: actualPath, Reason: "archived"}
		}
		if actualPath != expectedPath {
			return nil, &ProjectLocationConflictError{ProjectID: snapshotProject.ID, ExpectedPath: expectedPath, ActualPath: actualPath, Reason: "moved"}
		}
	}
	return &storage.ArtifactPrecondition{
		Path:             config.WorkspaceCatalogPath(dataDir),
		ExpectedRevision: revision,
	}, nil
}

func noteWorkspaceDataDir(configured string, projects []*models.Project) (string, error) {
	if strings.TrimSpace(configured) != "" {
		return filepath.Clean(configured), nil
	}
	var dataDir string
	for _, project := range projects {
		if project == nil {
			return "", invalidInput("note location", "resolved project is required")
		}
		projectsDir := filepath.Dir(filepath.Clean(project.Path))
		if filepath.Base(projectsDir) != "projects" {
			return "", invalidInput("note location", "project path is outside the canonical workspace projects directory")
		}
		candidate := filepath.Dir(projectsDir)
		if dataDir != "" && candidate != dataDir {
			return "", invalidInput("note location", "projects do not share one workspace data directory")
		}
		dataDir = candidate
	}
	if dataDir == "" {
		return "", invalidInput("note location", "project notes require a workspace data directory")
	}
	return dataDir, nil
}

func (p storageWorkspaceNotePort) noteDir(location NoteLocation, project *models.Project) (string, error) {
	switch location.Scope {
	case NoteScopeProject:
		if project == nil || project.ID != location.ProjectID {
			return "", invalidInput("note location", "resolved project is required")
		}
		return storage.NotesDir(project), nil
	case NoteScopeGlobal:
		if strings.TrimSpace(p.dataDir) == "" {
			return "", invalidInput("note location", "global notes require a workspace data directory")
		}
		return storage.GlobalNotesDir(p.dataDir), nil
	default:
		return "", invalidInput("note location", "scope must be project or global")
	}
}

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
	Kind      NoteMutationKind
	Location  NoteLocation
	ProjectID string // compatibility convenience; mirrors Location.ProjectID
	Filename  string
	Title     string
	Note      *models.Note
}

// NoteBatchResult reports every operation in request order and the shared
// transaction ID proving the batch committed atomically.
type NoteBatchResult struct {
	Results []NoteResult
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
	return result.Results[0], nil
}

// SaveNote is the project-scoped compatibility wrapper. A note with no observed
// RawRevision is a create; a loaded note is an update. New callers should prefer
// the explicit request methods above.
func (s *Service) SaveNote(projectID string, note *models.Note) (NoteResult, error) {
	location := ProjectNoteLocation(projectID)
	var (
		result NoteResult
		err    error
	)
	if note != nil && note.RawRevision != "" {
		result, err = s.UpdateNote(NoteUpdateRequest{Location: location, Note: note})
	} else {
		result, err = s.CreateNote(NoteCreateRequest{Location: location, Note: note})
	}
	if err == nil && note != nil && result.Note != nil {
		*note = *cloneNote(result.Note)
	}
	return result, err
}

// DeleteNote is the project-scoped compatibility wrapper. It first loads the
// source revision through the note port, then issues the typed delete.
func (s *Service) DeleteNote(projectID, filename string) (NoteResult, error) {
	if strings.TrimSpace(filename) == "" {
		return NoteResult{}, invalidInput("note", "filename is required")
	}
	location := ProjectNoteLocation(projectID)
	notes, err := s.NotesAt(location)
	if err != nil {
		return NoteResult{}, err
	}
	for _, note := range notes {
		if note.Filename == filename {
			return s.RemoveNote(NoteDeleteRequest{Location: location, Note: note})
		}
	}
	return NoteResult{}, fmt.Errorf("app: note not found: %s", filename)
}

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
		_, commitErr := committer.CommitArtifactTransaction(storage.ArtifactCommitRequest{
			Artifacts:     artifactChanges(artifacts),
			Preconditions: preconditions,
			ProjectIDs:    projectIDs,
			UndoAction:    action,
			UndoContext:   context,
		})
		if commitErr == nil {
			return NoteBatchResult{Results: results}, nil
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
			result.Note = note
			artifacts = append(artifacts, preparedNoteArtifact{change: change, operation: i, destination: true})
		case NoteUpdate:
			change, encErr := s.notes.EncodeNoteSave(op.source, sourceProject, note)
			if encErr != nil {
				return nil, nil, false, encErr
			}
			change.ExpectedRevision = stringPointer(op.note.RawRevision)
			note.RawRevision = storage.RawArtifactRevision(change.Data)
			result.Note = note
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
			result.Note = note
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
			result.Note = cloneNote(op.note)
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

func resolveNoteLocation(snap Snapshot, location NoteLocation) (*models.Project, error) {
	switch location.Scope {
	case NoteScopeGlobal:
		if location.ProjectID != "" {
			return nil, invalidInput("note location", "global scope cannot carry a project id")
		}
		return nil, nil
	case NoteScopeProject:
		if strings.TrimSpace(location.ProjectID) == "" {
			return nil, invalidInput("note location", "project id is required")
		}
		project := snap.ProjectByID(location.ProjectID)
		if project == nil {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, location.ProjectID)
		}
		return project, nil
	default:
		return nil, invalidInput("note location", "scope must be project or global")
	}
}

func requireObservedNote(note *models.Note) error {
	if strings.TrimSpace(note.Filename) == "" {
		return invalidInput("note", "filename is required")
	}
	if strings.TrimSpace(note.RawRevision) == "" {
		return invalidInput("note revision", "an observed source revision is required")
	}
	return nil
}

func cloneNote(note *models.Note) *models.Note {
	if note == nil {
		return nil
	}
	copyNote := *note
	return &copyNote
}

func collisionFilename(base string, index int) string {
	if index <= 1 {
		return base
	}
	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	return fmt.Sprintf("%s-%d%s", stem, index, extension)
}

func bumpCollision(operation *preparedNoteOperation) bool {
	if operation.collisionIndex >= maxNoteCollisionAttempts {
		return false
	}
	operation.collisionIndex++
	return true
}

func advanceCollision(operations []preparedNoteOperation, artifacts []preparedNoteArtifact, conflictPath string) bool {
	conflictPath = filepath.Clean(conflictPath)
	for i := range artifacts {
		artifact := artifacts[i]
		if artifact.destination && filepath.Clean(artifact.change.Path) == conflictPath {
			return bumpCollision(&operations[artifact.operation])
		}
	}
	return false
}

func artifactChanges(artifacts []preparedNoteArtifact) []storage.ArtifactChange {
	changes := make([]storage.ArtifactChange, len(artifacts))
	for i := range artifacts {
		changes[i] = artifacts[i].change
	}
	return changes
}

func noteProjectIDs(operations []preparedNoteOperation) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, operation := range operations {
		for _, location := range []NoteLocation{operation.source, operation.destination} {
			if location.Scope == NoteScopeProject && !seen[location.ProjectID] {
				seen[location.ProjectID] = true
				ids = append(ids, location.ProjectID)
			}
		}
	}
	return ids
}

func noteProjects(snap Snapshot, projectIDs []string) []*models.Project {
	projects := make([]*models.Project, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		if project := snap.ProjectByID(projectID); project != nil {
			projects = append(projects, project)
		}
	}
	return projects
}

func noteUndoMetadata(req NoteBatchRequest, results []NoteResult) (string, string) {
	action := strings.TrimSpace(req.UndoAction)
	if action == "" {
		if len(results) == 1 {
			action = strings.ToUpper(string(results[0].Kind[:1])) + string(results[0].Kind[1:]) + " Note"
		} else {
			action = "Update Notes"
		}
	}
	context := strings.TrimSpace(req.UndoContext)
	if context == "" && len(results) == 1 {
		context = results[0].Title
	}
	return action, context
}

func stringPointer(value string) *string { return &value }
