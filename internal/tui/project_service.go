package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectService provides consolidated project persistence operations for TUI handlers.
// It encapsulates the backup-save-reload pattern used throughout the application.
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
