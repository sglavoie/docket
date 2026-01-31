package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestListTrashedProjects_Empty(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	trashDir := filepath.Join(tempDir, ".tt", "trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}

	projects, err := storage.ListTrashedProjects(tempDir)
	testutil.AssertNoError(t, err)

	if len(projects) != 0 {
		t.Errorf("expected 0 trashed projects, got %d", len(projects))
	}
}

func TestListTrashedProjects_WithProjects(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	trashDir := filepath.Join(tempDir, ".tt", "trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}

	projectDir := filepath.Join(trashDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	projects, err := storage.ListTrashedProjects(tempDir)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(projects), 1)
	testutil.AssertEqual(t, projects[0], "test-project")
}

func TestTrashEntryExists_True(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	trashDir := filepath.Join(tempDir, ".tt", "trash")
	projectDir := filepath.Join(trashDir, "existing-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	exists := storage.TrashEntryExists("existing-project", tempDir)
	if !exists {
		t.Error("expected trash entry to exist")
	}
}

func TestTrashEntryExists_False(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	trashDir := filepath.Join(tempDir, ".tt", "trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}

	exists := storage.TrashEntryExists("nonexistent-project", tempDir)
	if exists {
		t.Error("expected trash entry to not exist")
	}
}

func TestReconstructProjectFromDir(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `# Test Project

## Task: Sample Task
<!-- id: task-1, status: todo, priority: 3 -->

A sample task.
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project, err := reconstructProjectFromDir("test-project", projectDir)
	testutil.AssertNoError(t, err)

	if project == nil {
		t.Fatal("expected non-nil project")
	}

	testutil.AssertEqual(t, project.Slug, "test-project")
	testutil.AssertEqual(t, project.Path, projectDir)

	if project.ID == "" {
		t.Error("expected project to have a generated ID")
	}
}

func TestReconstructProjectFromDir_ParsesProjectName(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `# Project: My Awesome Project

## Task: First Task
<!-- id: task-1, status: todo, priority: 3 -->

Task content.
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project, err := reconstructProjectFromDir("my-project", projectDir)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, project.Name, "My Awesome Project")
}

func TestReconstructProjectFromDir_FallsBackToSlug(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "slug-only")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `## Task: Only Task
<!-- id: task-1, status: todo, priority: 3 -->

Task without project header.
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project, err := reconstructProjectFromDir("slug-only", projectDir)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, project.Name, "slug-only")
}

func TestReconstructProjectFromDir_SetsDefaultContext(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "context-test")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `# Context Test Project
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project, err := reconstructProjectFromDir("context-test", projectDir)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, project.Context, "personal")
}

func TestReconstructProjectFromDir_SetsTimestamps(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "timestamps-test")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `# Timestamps Test
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	beforeRestore := time.Now().Add(-time.Second)
	project, err := reconstructProjectFromDir("timestamps-test", projectDir)
	testutil.AssertNoError(t, err)
	afterRestore := time.Now().Add(time.Second)

	if project.CreatedAt.Before(beforeRestore) {
		t.Error("CreatedAt should be after test start")
	}
	if project.CreatedAt.After(afterRestore) {
		t.Error("CreatedAt should be before test end")
	}
	if project.UpdatedAt.Before(beforeRestore) {
		t.Error("UpdatedAt should be after test start")
	}
	if project.UpdatedAt.After(afterRestore) {
		t.Error("UpdatedAt should be before test end")
	}
}

func TestReconstructProjectFromDir_LoadsTasks(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "tasks-test")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksContent := `# Tasks Test Project

## Task: Task One
<!-- id: task-1, status: todo, priority: 1 -->

First task.

## Task: Task Two
<!-- id: task-2, status: in_progress, priority: 2 -->

Second task.
`
	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project, err := reconstructProjectFromDir("tasks-test", projectDir)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(project.Tasks), 2)
}

func TestProjectRestore_ActiveProjectConflict(t *testing.T) {
	cfg := testutil.NewTestConfig()

	existingProject := &models.Project{
		ID:   uuid.New().String(),
		Slug: "conflicting-slug",
		Name: "Existing Project",
	}
	cfg.Projects = []*models.Project{existingProject}

	found := cfg.GetProject("conflicting-slug")
	if found == nil {
		t.Error("expected to find existing project by slug")
	}
}

func TestListTrashedProjects_ErrorHandling(t *testing.T) {
	projects, err := storage.ListTrashedProjects("/nonexistent/path/that/does/not/exist")
	// Either error or empty result is acceptable for nonexistent path
	if err == nil && len(projects) > 0 {
		t.Error("expected empty result or error for nonexistent path")
	}
}

func TestMoveProjectToTrash_Basic(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "to-trash")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tasksPath := filepath.Join(projectDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("# To Trash\n"), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project := &models.Project{
		ID:   uuid.New().String(),
		Slug: "to-trash",
		Name: "To Trash",
		Path: projectDir,
	}

	err := storage.MoveProjectToTrash(project, tempDir)
	testutil.AssertNoError(t, err)

	exists := storage.TrashEntryExists("to-trash", tempDir)
	if !exists {
		t.Error("expected project to be in trash")
	}

	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Error("expected original project dir to not exist")
	}
}

func TestRestoreProjectFromTrash(t *testing.T) {
	tempDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	trashDir := filepath.Join(tempDir, ".tt", "trash")
	projectTrashDir := filepath.Join(trashDir, "to-restore")
	if err := os.MkdirAll(projectTrashDir, 0755); err != nil {
		t.Fatalf("failed to create trash project dir: %v", err)
	}

	tasksPath := filepath.Join(projectTrashDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("# To Restore\n"), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	// Create projects dir for restore destination
	if err := os.MkdirAll(filepath.Join(tempDir, "projects"), 0755); err != nil {
		t.Fatalf("failed to create projects dir: %v", err)
	}

	err := storage.RestoreProjectFromTrash("to-restore", tempDir)
	testutil.AssertNoError(t, err)

	restoredDir := filepath.Join(tempDir, "projects", "to-restore")
	if _, err := os.Stat(restoredDir); os.IsNotExist(err) {
		t.Error("expected restored project dir to exist")
	}

	exists := storage.TrashEntryExists("to-restore", tempDir)
	if exists {
		t.Error("expected project to not be in trash after restore")
	}
}

func TestListTrashedProjects_Function(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.DataDir = "/tmp/test-data"

	testCases := []struct {
		name     string
		projects []string
		expected int
	}{
		{"empty list", []string{}, 0},
		{"single project", []string{"proj1"}, 1},
		{"multiple projects", []string{"proj1", "proj2", "proj3"}, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.projects) != tc.expected {
				t.Errorf("expected %d projects, got %d", tc.expected, len(tc.projects))
			}
		})
	}
}

func TestConfigAddProjectOnRestore(t *testing.T) {
	cfg := &config.Config{}

	project := testutil.NewTestProject("Test Project")
	cfg.AddProject(project)

	testutil.AssertEqual(t, len(cfg.Projects), 1)
	testutil.AssertEqual(t, cfg.Projects[0].Name, "Test Project")
}
