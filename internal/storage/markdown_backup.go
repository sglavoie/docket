package storage

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// BackupExists checks if a backup file exists for the project.
func BackupExists(project *models.Project) bool {
	_, err := os.Stat(backupFilePath(project.Path))
	return err == nil
}

// BackupProject creates a backup of the current tasks.md file.
// Only creates a backup if tasks.md exists (no backup for new projects).
func BackupProject(project *models.Project) error {
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks file for backup: %w", err)
	}

	if err := writeFileAtomic(backupFilePath(project.Path), content, FilePermissions); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// RestoreFromBackup swaps the current tasks file with the backup.
// This makes the operation reversible - running restore again undoes the restore.
func RestoreFromBackup(project *models.Project) error {
	backupPath := backupFilePath(project.Path)
	tasksPath := tasksFilePath(project.Path)

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", tterrors.ErrBackupNotFound, backupPath)
		}
		return fmt.Errorf("%w: %w", tterrors.ErrReadFailed, err)
	}

	if _, err := ParseTasksMarkdownWithSections(string(backupContent)); err != nil {
		return fmt.Errorf("backup file is invalid: %w", err)
	}

	currentContent, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		currentContent = []byte{}
	} else if err != nil {
		return fmt.Errorf("failed to read current tasks file: %w", err)
	}

	if err := writeFileAtomic(tasksPath, backupContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to restore tasks file: %w", err)
	}
	if err := writeFileAtomic(backupPath, currentContent, FilePermissions); err != nil {
		return fmt.Errorf("failed to update backup file: %w", err)
	}

	return nil
}
