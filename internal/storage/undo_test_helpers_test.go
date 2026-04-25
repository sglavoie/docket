package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newUndoTestProject(t *testing.T) (*models.Project, string) {
	t.Helper()

	projectPath := filepath.Join(t.TempDir(), "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Name: "Test Project",
		Slug: "test-project",
		Path: projectPath,
	}

	return project, filepath.Join(projectPath, "tasks.md")
}

func writeUndoTasks(t *testing.T, tasksPath, content string) {
	t.Helper()

	if err := os.WriteFile(tasksPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
}