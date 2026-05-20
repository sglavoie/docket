package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestArchiveTasks_RollbackRemovesEmptyArchiveWhenAbsent verifies that when the
// archive did not exist before ArchiveTasks and the SaveProject step fails, the
// rollback removes the freshly created archive file instead of leaving an empty
// 0-byte completed.md behind. Truncating a never-existed file to 0 bytes creates
// a stray empty archive that pollutes the project directory.
func TestArchiveTasks_RollbackRemovesEmptyArchiveWhenAbsent(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "rollback-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	now := time.Now()
	project := &models.Project{
		ID:   uuid.New().String(),
		Name: "Rollback Project",
		Slug: "rollback-project",
		Path: projectPath,
		Tasks: []*models.Task{
			{ID: "done-1", ShortID: 1, Title: "Done", Type: models.TaskTypeTask, Status: models.StatusDone, Priority: 3, CompletedAt: &now},
		},
		NextShortID: 2,
	}

	// Confirm archive does not exist before the operation.
	archivePath := filepath.Join(projectPath, archiveFileName)
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive should not exist before test, stat err=%v", err)
	}

	// Replace writeFileAtomic with a stub that fails on the project's tasks.md
	// path (forcing the SaveProject step to fail) but succeeds for the archive
	// rollback's own writes (none, since rollback uses os.Truncate/os.Remove).
	tasksPath := tasksFilePath(projectPath)
	origWrite := writeFileAtomic
	writeFileAtomic = func(path string, data []byte, perm os.FileMode) error {
		if path == tasksPath {
			return errors.New("simulated save failure")
		}
		return origWrite(path, data, perm)
	}
	defer func() { writeFileAtomic = origWrite }()

	_, err := ArchiveTasks(project)
	if err == nil {
		t.Fatal("expected ArchiveTasks to return error when SaveProject fails")
	}

	// The archive file should NOT exist after rollback: it was created during
	// append (since it did not exist before) and must be removed, not merely
	// truncated to zero bytes.
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Errorf("rollback should remove the newly created archive file when it did not exist before; stat err=%v", statErr)
	}
}
