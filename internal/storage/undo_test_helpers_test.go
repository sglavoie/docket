package storage

import (
	"fmt"
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

type undoTestProjectFixture struct {
	Project   *models.Project
	TasksPath string
}

func newUndoTestProjects(t *testing.T, slugs ...string) []undoTestProjectFixture {
	t.Helper()

	root := t.TempDir()
	fixtures := make([]undoTestProjectFixture, 0, len(slugs))
	for i, slug := range slugs {
		if slug == "" {
			slug = fmt.Sprintf("project-%d", i+1)
		}
		projectPath := filepath.Join(root, slug)
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatalf("failed to create project dir %q: %v", slug, err)
		}

		project := &models.Project{
			ID:   fmt.Sprintf("%s-id", slug),
			Name: fmt.Sprintf("Test Project %d", i+1),
			Slug: slug,
			Path: projectPath,
		}
		fixtures = append(fixtures, undoTestProjectFixture{
			Project:   project,
			TasksPath: filepath.Join(projectPath, "tasks.md"),
		})
	}

	return fixtures
}

func writeUndoTasks(t *testing.T, tasksPath, content string) {
	t.Helper()

	if err := os.WriteFile(tasksPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}
}

func readUndoMetaForTest(t *testing.T, project *models.Project) *UndoMeta {
	t.Helper()

	meta, err := loadUndoMeta(project)
	if err != nil {
		t.Fatalf("loadUndoMeta(%s) failed: %v", project.Slug, err)
	}
	return meta
}

func assertUndoMetaCounts(t *testing.T, meta *UndoMeta, undoEntries, redoEntries int) {
	t.Helper()

	if len(meta.Entries) != undoEntries {
		t.Fatalf("undo entries = %d, want %d", len(meta.Entries), undoEntries)
	}
	if len(meta.RedoEntries) != redoEntries {
		t.Fatalf("redo entries = %d, want %d", len(meta.RedoEntries), redoEntries)
	}
}
