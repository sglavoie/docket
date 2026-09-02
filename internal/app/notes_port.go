package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
