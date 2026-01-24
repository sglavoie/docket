package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetTrashDir(t *testing.T) {
	dataDir := "/home/user/.tt-data"
	trashDir := GetTrashDir(dataDir)
	expected := "/home/user/.tt-data/.trash"

	if trashDir != expected {
		t.Errorf("expected trash dir '%s', got '%s'", expected, trashDir)
	}
}

func TestMoveProjectToTrash(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	projectDir := filepath.Join(dataDir, "my-project")

	// Create project directory with tasks.md
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	tasksContent := "# Project: My Project\n"
	if err := os.WriteFile(filepath.Join(projectDir, "tasks.md"), []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Slug: "my-project",
		Name: "My Project",
		Path: projectDir,
	}

	// Move to trash
	if err := MoveProjectToTrash(project, dataDir); err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	// Original directory should not exist
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Error("original project directory should be removed")
	}

	// Trash directory should exist with content
	trashPath := filepath.Join(dataDir, ".trash", "my-project")
	if _, err := os.Stat(trashPath); os.IsNotExist(err) {
		t.Error("project should exist in trash")
	}

	// tasks.md should exist in trash
	trashedTasks := filepath.Join(trashPath, "tasks.md")
	content, err := os.ReadFile(trashedTasks)
	if err != nil {
		t.Fatalf("failed to read trashed tasks.md: %v", err)
	}
	if string(content) != tasksContent {
		t.Error("tasks.md content should be preserved in trash")
	}
}

func TestMoveProjectToTrash_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	projectDir := filepath.Join(dataDir, "my-project")
	trashPath := filepath.Join(dataDir, ".trash", "my-project")

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create existing trash entry
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Slug: "my-project",
		Name: "My Project",
		Path: projectDir,
	}

	// Move should fail when trash entry already exists (callers should purge first)
	err := MoveProjectToTrash(project, dataDir)
	if err == nil {
		t.Error("expected error when trash entry already exists")
	}
}

func TestPurgeFromTrash(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	trashPath := filepath.Join(dataDir, ".trash", "my-project")

	// Create trash entry with content
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trashPath, "tasks.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Purge should succeed
	if err := PurgeFromTrash("my-project", dataDir); err != nil {
		t.Fatalf("PurgeFromTrash failed: %v", err)
	}

	// Trash entry should no longer exist
	if TrashEntryExists("my-project", dataDir) {
		t.Error("trash entry should be removed after purge")
	}
}

func TestPurgeFromTrash_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")

	// Purge non-existent entry should succeed (no-op)
	if err := PurgeFromTrash("non-existent", dataDir); err != nil {
		t.Errorf("PurgeFromTrash should not error for non-existent entry: %v", err)
	}
}

func TestPurgeAndMoveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	projectDir := filepath.Join(dataDir, "my-project")
	trashPath := filepath.Join(dataDir, ".trash", "my-project")

	// Create old trash entry (simulating a previously deleted project)
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		t.Fatalf("failed to create old trash entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trashPath, "tasks.md"), []byte("# Old Project"), 0644); err != nil {
		t.Fatalf("failed to write old tasks.md: %v", err)
	}

	// Create new project with same slug
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	newContent := "# New Project\n"
	if err := os.WriteFile(filepath.Join(projectDir, "tasks.md"), []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write new tasks.md: %v", err)
	}

	project := &models.Project{
		ID:   "new-id",
		Slug: "my-project",
		Name: "New Project",
		Path: projectDir,
	}

	// Purge old trash entry
	if err := PurgeFromTrash(project.Slug, dataDir); err != nil {
		t.Fatalf("PurgeFromTrash failed: %v", err)
	}

	// Move new project to trash
	if err := MoveProjectToTrash(project, dataDir); err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	// Verify new content is in trash
	content, err := os.ReadFile(filepath.Join(trashPath, "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read trashed tasks.md: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("expected new content in trash, got: %s", string(content))
	}
}

func TestRestoreProjectFromTrash(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	trashPath := filepath.Join(dataDir, ".trash", "my-project")
	projectDir := filepath.Join(dataDir, "my-project")

	// Create trash entry with tasks.md
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}
	tasksContent := "# Project: My Project\n"
	if err := os.WriteFile(filepath.Join(trashPath, "tasks.md"), []byte(tasksContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	// Restore from trash
	if err := RestoreProjectFromTrash("my-project", dataDir); err != nil {
		t.Fatalf("RestoreProjectFromTrash failed: %v", err)
	}

	// Trash entry should not exist
	if _, err := os.Stat(trashPath); !os.IsNotExist(err) {
		t.Error("trash entry should be removed")
	}

	// Project directory should exist
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Error("project directory should be restored")
	}

	// tasks.md should exist in restored location
	restoredTasks := filepath.Join(projectDir, "tasks.md")
	content, err := os.ReadFile(restoredTasks)
	if err != nil {
		t.Fatalf("failed to read restored tasks.md: %v", err)
	}
	if string(content) != tasksContent {
		t.Error("tasks.md content should be preserved after restore")
	}
}

func TestRestoreProjectFromTrash_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")

	// Ensure data dir exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Restore non-existent project should fail
	err := RestoreProjectFromTrash("non-existent", dataDir)
	if err == nil {
		t.Error("expected error when restoring non-existent project")
	}
}

func TestRestoreProjectFromTrash_ConflictWithActive(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	trashPath := filepath.Join(dataDir, ".trash", "my-project")
	projectDir := filepath.Join(dataDir, "my-project")

	// Create both trash entry and active project directory
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Restore should fail when active project exists
	err := RestoreProjectFromTrash("my-project", dataDir)
	if err == nil {
		t.Error("expected error when active project exists")
	}
}

func TestTrashEntryExists(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	trashDir := filepath.Join(dataDir, ".trash")

	// No trash dir yet
	if TrashEntryExists("my-project", dataDir) {
		t.Error("expected false when trash dir doesn't exist")
	}

	// Create trash dir without the project
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}
	if TrashEntryExists("my-project", dataDir) {
		t.Error("expected false when project not in trash")
	}

	// Create project in trash
	if err := os.MkdirAll(filepath.Join(trashDir, "my-project"), 0755); err != nil {
		t.Fatalf("failed to create project in trash: %v", err)
	}
	if !TrashEntryExists("my-project", dataDir) {
		t.Error("expected true when project exists in trash")
	}
}

func TestListTrashedProjects(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")

	// No trash dir yet
	projects, err := ListTrashedProjects(dataDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}

	// Create trash dir with some projects
	trashDir := filepath.Join(dataDir, ".trash")
	if mkdirErr := os.MkdirAll(filepath.Join(trashDir, "project-a"), 0755); mkdirErr != nil {
		t.Fatalf("failed to create project-a: %v", mkdirErr)
	}
	if mkdirErr := os.MkdirAll(filepath.Join(trashDir, "project-b"), 0755); mkdirErr != nil {
		t.Fatalf("failed to create project-b: %v", mkdirErr)
	}
	if mkdirErr := os.MkdirAll(filepath.Join(trashDir, "project-c"), 0755); mkdirErr != nil {
		t.Fatalf("failed to create project-c: %v", mkdirErr)
	}

	// Create a file (should be ignored, only directories count)
	if writeErr := os.WriteFile(filepath.Join(trashDir, "not-a-project.txt"), []byte("test"), 0644); writeErr != nil {
		t.Fatalf("failed to create file: %v", writeErr)
	}

	projects, err = ListTrashedProjects(dataDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}

	// Check that all expected projects are listed
	projectMap := make(map[string]bool)
	for _, p := range projects {
		projectMap[p] = true
	}
	for _, expected := range []string{"project-a", "project-b", "project-c"} {
		if !projectMap[expected] {
			t.Errorf("expected project '%s' in list", expected)
		}
	}
}

func TestMoveAndRestoreRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data")
	projectDir := filepath.Join(dataDir, "my-project")

	// Create project with content
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	originalContent := "# Project: My Project\n\n## Task: Test Task\n<!-- id: task1, status: todo, priority: 1 -->\n"
	if err := os.WriteFile(filepath.Join(projectDir, "tasks.md"), []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	project := &models.Project{
		ID:   "test-id",
		Slug: "my-project",
		Name: "My Project",
		Path: projectDir,
	}

	// Move to trash
	if err := MoveProjectToTrash(project, dataDir); err != nil {
		t.Fatalf("MoveProjectToTrash failed: %v", err)
	}

	// Project should be in trash
	if !TrashEntryExists("my-project", dataDir) {
		t.Error("project should be in trash after move")
	}

	// Restore from trash
	if err := RestoreProjectFromTrash("my-project", dataDir); err != nil {
		t.Fatalf("RestoreProjectFromTrash failed: %v", err)
	}

	// Project should not be in trash
	if TrashEntryExists("my-project", dataDir) {
		t.Error("project should not be in trash after restore")
	}

	// Content should be preserved
	content, err := os.ReadFile(filepath.Join(projectDir, "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read restored tasks.md: %v", err)
	}
	if string(content) != originalContent {
		t.Error("content should be preserved after round trip")
	}
}
