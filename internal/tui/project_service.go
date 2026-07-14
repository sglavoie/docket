package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workspace"
)

// Persister defines the interface for project persistence operations.
// This interface enables dependency injection and testing by allowing
// mock implementations to replace the real storage layer.
//
// User-visible task/project mutations must not persist through this interface's
// raw Save; they commit through the application-layer TaskItemCoordinator /
// tuiAppRepository transaction boundary. The surviving raw helpers here back the
// undo/redo restore path and the archive flow, which are privileged and audited
// in internal/storage/undo_raw_save_audit_test.go.
type Persister interface {
	// Load reloads a project from disk.
	Load(project *models.Project) error

	// Save saves a project to disk without backup or reload.
	Save(project *models.Project) error

	// Backup creates a backup of the project without saving.
	Backup(project *models.Project) error

	// BackupExists checks if a backup file exists for the project.
	BackupExists(project *models.Project) bool

	// RestoreFromBackup restores a project from its backup file.
	RestoreFromBackup(project *models.Project) error

	// RecordUndoSnapshot creates an undo snapshot for the project.
	// taskIDs identifies the tasks explicitly changed by the user; when non-nil,
	// undo will only restore those tasks instead of replacing the entire file.
	RecordUndoSnapshot(project *models.Project, action, context string, depth int, taskIDs []string) error

	// RecordUndoSnapshotWithOptions creates an undo snapshot with optional
	// targeted task and grouped transaction metadata.
	RecordUndoSnapshotWithOptions(project *models.Project, opts storage.UndoSnapshotOptions) error

	// ArchiveTasks moves completed tasks to the archive file.
	ArchiveTasks(project *models.Project) (int, error)
}

// ProjectService provides consolidated project persistence operations for TUI handlers.
// It encapsulates the load/backup/save/undo primitives used by the undo/redo
// restore path and the archive flow. ProjectService implements the Persister interface.
type ProjectService struct {
	workspaceLoader *workspace.Loader
}

// NewProjectService creates a new ProjectService instance.
func NewProjectService() *ProjectService {
	return &ProjectService{workspaceLoader: workspace.NewLoader()}
}

// Load reloads a project from disk.
func (s *ProjectService) Load(project *models.Project) error {
	return storage.LoadProject(project)
}

// BackupExists checks if a backup file exists for the project.
func (s *ProjectService) BackupExists(project *models.Project) bool {
	return storage.BackupExists(project)
}

// RestoreFromBackup restores a project from its backup file.
func (s *ProjectService) RestoreFromBackup(project *models.Project) error {
	return storage.RestoreFromBackup(project)
}

// RecordUndoSnapshot creates an undo snapshot for the project.
// Returns an error if the snapshot cannot be created.
func (s *ProjectService) RecordUndoSnapshot(project *models.Project, action, context string, depth int, taskIDs []string) error {
	return storage.RecordUndoSnapshot(project, action, context, depth, taskIDs)
}

// RecordUndoSnapshotWithOptions creates an undo snapshot for the project.
// Returns an error if the snapshot cannot be created.
func (s *ProjectService) RecordUndoSnapshotWithOptions(project *models.Project, opts storage.UndoSnapshotOptions) error {
	return storage.RecordUndoSnapshotWithOptions(project, opts)
}

// ArchiveTasks moves completed tasks to the archive file.
// Returns the number of tasks archived and any error encountered.
func (s *ProjectService) ArchiveTasks(project *models.Project) (int, error) {
	return storage.ArchiveTasks(project)
}

// Backup creates a backup of the project without saving.
func (s *ProjectService) Backup(project *models.Project) error {
	return storage.BackupProject(project)
}

// Save saves a project to disk without backup or reload. It is the raw restore
// save used by the undo/redo reapply path (reapplySideEffects); user mutations
// commit through the application-layer transaction boundary instead.
func (s *ProjectService) Save(project *models.Project) error {
	return storage.SaveProjectLocked(project)
}

// defaultPersister is a package-level Persister for callers that don't hold
// their own ProjectService instance (e.g. daily/weekly review views).
var defaultPersister Persister = &ProjectService{}
