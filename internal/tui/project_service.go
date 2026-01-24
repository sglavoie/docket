package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	RecordUndoSnapshot(project *models.Project, action, context string, depth int) error

	// ArchiveTasks moves completed tasks to the archive file.
	ArchiveTasks(project *models.Project) (int, error)

	// SaveWithBackup returns a command that saves a project asynchronously.
	SaveWithBackup(project *models.Project, successMsg string) tea.Cmd

	// SaveMultiple returns a command that saves multiple projects asynchronously.
	SaveMultiple(projects []*models.Project, successMsg string) tea.Cmd
}

// ProjectService provides consolidated project persistence operations for TUI handlers.
// It encapsulates the backup-save-reload pattern used throughout the application.
// ProjectService implements the Persister interface.
type ProjectService struct{}

// NewProjectService creates a new ProjectService instance.
func NewProjectService() *ProjectService {
	return &ProjectService{}
}

// SaveWithBackup returns a command that saves a project asynchronously.
// It backs up the project first, then saves it, and reloads to ensure fresh state.
// On success, it returns SaveCompleteMsg with the provided success message.
// On failure, it returns SaveErrorMsg with the error.
func (s *ProjectService) SaveWithBackup(project *models.Project, successMsg string) tea.Cmd {
	return func() tea.Msg {
		if err := storage.BackupProject(project); err != nil {
			return SaveErrorMsg{Err: err}
		}
		if err := storage.SaveProject(project); err != nil {
			return SaveErrorMsg{Err: err}
		}
		_ = storage.LoadProject(project)
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// SaveMultiple returns a command that saves multiple projects asynchronously.
// Each project is backed up, saved, and reloaded. If any operation fails,
// the command returns SaveErrorMsg immediately.
func (s *ProjectService) SaveMultiple(projects []*models.Project, successMsg string) tea.Cmd {
	return func() tea.Msg {
		for _, project := range projects {
			if err := storage.BackupProject(project); err != nil {
				return SaveErrorMsg{Err: err}
			}
			if err := storage.SaveProject(project); err != nil {
				return SaveErrorMsg{Err: err}
			}
			_ = storage.LoadProject(project)
		}
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// SaveWithBackupSync saves a project synchronously with backup.
// Returns an error if backup or save fails. Use this when you need
// to continue processing after save within the same handler.
func (s *ProjectService) SaveWithBackupSync(project *models.Project) error {
	if err := storage.BackupProject(project); err != nil {
		return err
	}
	if err := storage.SaveProject(project); err != nil {
		return err
	}
	_ = storage.LoadProject(project)
	return nil
}

// SaveMultipleSync saves multiple projects synchronously with backup.
// Returns an error if any backup or save fails.
func (s *ProjectService) SaveMultipleSync(projects []*models.Project) error {
	for _, project := range projects {
		if err := storage.BackupProject(project); err != nil {
			return err
		}
		if err := storage.SaveProject(project); err != nil {
			return err
		}
		_ = storage.LoadProject(project)
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
func (s *ProjectService) RecordUndoSnapshot(project *models.Project, action, context string, depth int) error {
	return storage.RecordUndoSnapshot(project, action, context, depth)
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
	return storage.SaveProject(project)
}

// Package-level convenience functions for common use cases

// saveProjectSync saves a project synchronously with backup and reload.
// Use when you need to continue processing after save.
func saveProjectSync(project *models.Project) error {
	if err := storage.BackupProject(project); err != nil {
		return err
	}
	if err := storage.SaveProject(project); err != nil {
		return err
	}
	_ = storage.LoadProject(project)
	return nil
}
