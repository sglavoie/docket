package tui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// Persister defines the interface for project persistence operations.
// This interface enables dependency injection and testing by allowing
// mock implementations to replace the real storage layer.
type Persister interface {
	// Load reloads a project from disk.
	Load(project *models.Project) error

	// Save saves a project to disk without backup or reload.
	Save(project *models.Project) error

	// Backup creates a backup of the project without saving.
	Backup(project *models.Project) error

	// SaveWithBackupSync saves a project synchronously with backup and reload.
	SaveWithBackupSync(project *models.Project) error

	// SaveMultipleSync saves multiple projects synchronously with backup and reload.
	SaveMultipleSync(projects []*models.Project) error

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

	// SaveWithBackupSyncOptions saves a project synchronously with backup,
	// reload, and optional undo metadata.
	SaveWithBackupSyncOptions(project *models.Project, opts ProjectSaveOptions) error

	// SaveMultipleSyncOptions saves multiple projects synchronously with backup,
	// reload, and optional undo metadata.
	SaveMultipleSyncOptions(projects []*models.Project, opts ProjectSaveOptions) error

	// ArchiveTasks moves completed tasks to the archive file.
	ArchiveTasks(project *models.Project) (int, error)

	// SaveWithBackup returns a command that saves a project asynchronously.
	SaveWithBackup(project *models.Project, successMsg string) tea.Cmd

	// SaveWithBackupOptions returns a command that saves a project
	// asynchronously with optional undo metadata.
	SaveWithBackupOptions(project *models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd

	// SaveMultiple returns a command that saves multiple projects asynchronously.
	SaveMultiple(projects []*models.Project, successMsg string) tea.Cmd

	// SaveMultipleOptions returns a command that saves multiple projects
	// asynchronously with optional undo metadata.
	SaveMultipleOptions(projects []*models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd
}

// ProjectSaveOptions carries optional undo metadata for TUI save helpers.
// Empty undo fields keep legacy save behavior for maintenance paths that are
// intentionally not undoable yet.
type ProjectSaveOptions struct {
	UndoAction          string
	UndoContext         string
	UndoDepth           int
	UndoTaskIDs         []string
	UndoGroupID         string
	UndoGroupProjectIDs []string
}

func (opts ProjectSaveOptions) hasUndoMetadata() bool {
	return opts.UndoAction != "" ||
		opts.UndoContext != "" ||
		len(opts.UndoTaskIDs) > 0 ||
		opts.UndoGroupID != "" ||
		len(opts.UndoGroupProjectIDs) > 0
}

func (opts ProjectSaveOptions) storageUndoOptions() storage.UndoSnapshotOptions {
	action := opts.UndoAction
	if action == "" {
		action = "Task Modified"
	}
	return storage.UndoSnapshotOptions{
		Action:          action,
		Context:         opts.UndoContext,
		MaxDepth:        opts.UndoDepth,
		TaskIDs:         opts.UndoTaskIDs,
		GroupID:         opts.UndoGroupID,
		GroupProjectIDs: opts.UndoGroupProjectIDs,
	}
}

func recordUndoSnapshotForSave(project *models.Project, opts ProjectSaveOptions) {
	if opts.UndoDepth <= 0 || !opts.hasUndoMetadata() {
		return
	}
	if err := storage.RecordUndoSnapshotWithOptions(project, opts.storageUndoOptions()); err != nil {
		logging.Warn("failed to record undo snapshot: %v", err)
	}
}

// ProjectService provides consolidated project persistence operations for TUI handlers.
// It encapsulates the backup-save-reload pattern used throughout the application.
// ProjectService implements the Persister interface.
type ProjectService struct{}

// NewProjectService creates a new ProjectService instance.
func NewProjectService() *ProjectService {
	return &ProjectService{}
}

// lockPath returns the advisory lock file path for a project's tasks file.
func lockPath(project *models.Project) string {
	return filepath.Join(project.Path, storage.LockFileName)
}

// SaveWithBackup returns a command that saves a project asynchronously.
// It backs up the project first, then saves it, and reloads to ensure fresh state.
// On success, it returns SaveCompleteMsg with the provided success message.
// On failure, it returns SaveErrorMsg with the error.
func (s *ProjectService) SaveWithBackup(project *models.Project, successMsg string) tea.Cmd {
	return s.SaveWithBackupOptions(project, successMsg, ProjectSaveOptions{})
}

// SaveWithBackupOptions returns a command that saves a project asynchronously
// with optional undo metadata.
func (s *ProjectService) SaveWithBackupOptions(project *models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd {
	return func() tea.Msg {
		if err := saveProjectCore(project, true, opts); err != nil {
			return SaveErrorMsg{Err: err}
		}
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// SaveMultiple returns a command that saves multiple projects asynchronously.
// Each project is backed up, saved, and reloaded. If any operation fails,
// the command returns SaveErrorMsg immediately.
func (s *ProjectService) SaveMultiple(projects []*models.Project, successMsg string) tea.Cmd {
	return s.SaveMultipleOptions(projects, successMsg, ProjectSaveOptions{})
}

// SaveMultipleOptions returns a command that saves multiple projects
// asynchronously with optional undo metadata.
func (s *ProjectService) SaveMultipleOptions(projects []*models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd {
	return func() tea.Msg {
		for _, project := range projects {
			if err := saveProjectCore(project, true, opts); err != nil {
				return SaveErrorMsg{Err: err}
			}
		}
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// SaveWithBackupSync saves a project synchronously with backup.
// Returns an error if backup or save fails. Use this when you need
// to continue processing after save within the same handler.
func (s *ProjectService) SaveWithBackupSync(project *models.Project) error {
	return s.SaveWithBackupSyncOptions(project, ProjectSaveOptions{})
}

// SaveWithBackupSyncOptions saves a project synchronously with backup and
// optional undo metadata.
func (s *ProjectService) SaveWithBackupSyncOptions(project *models.Project, opts ProjectSaveOptions) error {
	return saveProjectCore(project, true, opts)
}

// SaveMultipleSync saves multiple projects synchronously with backup.
// Returns an error if any backup or save fails.
func (s *ProjectService) SaveMultipleSync(projects []*models.Project) error {
	return s.SaveMultipleSyncOptions(projects, ProjectSaveOptions{})
}

// SaveMultipleSyncOptions saves multiple projects synchronously with backup and
// optional undo metadata.
func (s *ProjectService) SaveMultipleSyncOptions(projects []*models.Project, opts ProjectSaveOptions) error {
	for _, project := range projects {
		if err := saveProjectCore(project, true, opts); err != nil {
			return err
		}
	}
	return nil
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

// Save saves a project to disk without backup or reload.
func (s *ProjectService) Save(project *models.Project) error {
	return storage.SaveProjectLocked(project)
}

// defaultPersister is a package-level Persister for callers that don't hold
// their own ProjectService instance (e.g. daily/weekly review views).
var defaultPersister Persister = &ProjectService{}

// saveProjectCore is the single implementation of the backup-save-reload pattern.
// All save functions delegate to this. When reload is true, the project is
// reloaded from disk after saving to ensure fresh state.
func saveProjectCore(project *models.Project, reload bool, opts ProjectSaveOptions) error {
	return util.WithFileLock(lockPath(project), true, func() error {
		recordUndoSnapshotForSave(project, opts)
		if err := storage.BackupProject(project); err != nil {
			return err
		}
		if err := storage.SaveProject(project); err != nil {
			return err
		}
		if reload {
			return storage.LoadProject(project)
		}
		return nil
	})
}

// Package-level convenience functions for common use cases

// saveProjectSync saves a project synchronously with backup and reload.
// Use when you need to continue processing after save.
func saveProjectSync(project *models.Project) error {
	return saveProjectCore(project, true, ProjectSaveOptions{})
}

// saveProjectNoReload saves a project with backup and file locking but without
// reloading from disk. Safe to call from tea.Cmd goroutines since it does not
// replace the in-memory task objects.
func saveProjectNoReload(project *models.Project) error {
	return saveProjectCore(project, false, ProjectSaveOptions{})
}
