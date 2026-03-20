package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSaveProject_Roundtrip(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProjectWithTasks("Test Project")
	project.Path = filepath.Join(dir, "test-project")

	// Save the project
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Verify file exists
	tasksPath := filepath.Join(project.Path, "tasks.md")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Fatal("tasks.md was not created")
	}

	// Create a new project and load the saved data
	loadedProject := testutil.NewTestProject("Test Project")
	loadedProject.Path = project.Path

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	// Verify task count matches
	if len(loadedProject.Tasks) != len(project.Tasks) {
		t.Errorf("expected %d tasks, got %d", len(project.Tasks), len(loadedProject.Tasks))
	}

	// Verify task hierarchy is preserved
	for _, task := range loadedProject.Tasks {
		original := findTaskByID(project.Tasks, task.ID)
		if original == nil {
			t.Errorf("task %s not found in original", task.ID)
			continue
		}

		if task.Title != original.Title {
			t.Errorf("task %s: expected title %q, got %q", task.ID, original.Title, task.Title)
		}
		if task.Type != original.Type {
			t.Errorf("task %s: expected type %s, got %s", task.ID, original.Type, task.Type)
		}
		if task.Status != original.Status {
			t.Errorf("task %s: expected status %s, got %s", task.ID, original.Status, task.Status)
		}
		if task.Priority != original.Priority {
			t.Errorf("task %s: expected priority %d, got %d", task.ID, original.Priority, task.Priority)
		}
	}
}

func TestSaveProject_WithSections(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Sections Test")
	project.Path = filepath.Join(dir, "sections-test")
	project.Sections = []string{"Next", "Active", "Done"}
	project.SectionColors = map[string]string{
		"Active": "green",
		"Done":   "blue",
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Active"
	project.Tasks = []*models.Task{task}

	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Load and verify
	loadedProject := testutil.NewTestProject("Sections Test")
	loadedProject.Path = project.Path

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	// Original 3 sections + built-in defaults added by EnsureDefaultSections
	if len(loadedProject.Sections) < 3 {
		t.Errorf("expected at least 3 sections, got %d", len(loadedProject.Sections))
	}

	if loadedProject.SectionColors["Active"] != "green" {
		t.Errorf("expected Active section color green, got %s", loadedProject.SectionColors["Active"])
	}

	if len(loadedProject.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(loadedProject.Tasks))
	}

	if loadedProject.Tasks[0].CustomSection != "Active" {
		t.Errorf("expected task custom section Active, got %s", loadedProject.Tasks[0].CustomSection)
	}
}

func TestSaveProject_EmptyProject(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Empty Project")
	project.Path = filepath.Join(dir, "empty-project")
	project.Tasks = []*models.Task{}

	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Load and verify
	loadedProject := testutil.NewTestProject("Empty Project")
	loadedProject.Path = project.Path

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(loadedProject.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(loadedProject.Tasks))
	}
}

func TestBackupProject(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProjectWithTasks("Backup Test")
	project.Path = filepath.Join(dir, "backup-test")

	// Save initial project
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Create backup
	if err := BackupProject(project); err != nil {
		t.Fatalf("BackupProject failed: %v", err)
	}

	// Verify backup file exists
	backupPath := GetBackupFilePath(project)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("backup file was not created")
	}

	// Verify BackupExists returns true
	if !BackupExists(project) {
		t.Error("BackupExists returned false but backup exists")
	}

	// Read both files and verify content matches
	tasksContent, err := os.ReadFile(GetTasksFilePath(project))
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}

	if string(tasksContent) != string(backupContent) {
		t.Error("backup content does not match original")
	}
}

func TestBackupProject_NoTasksFile(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("No Tasks")
	project.Path = filepath.Join(dir, "no-tasks")

	// Don't save - no tasks.md exists
	// Backup should succeed silently
	if err := BackupProject(project); err != nil {
		t.Fatalf("BackupProject should not fail for non-existent tasks file: %v", err)
	}

	// Verify no backup was created
	if BackupExists(project) {
		t.Error("backup should not exist for non-existent tasks file")
	}
}

func TestRestoreFromBackup(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProjectWithTasks("Restore Test")
	project.Path = filepath.Join(dir, "restore-test")
	originalTaskCount := len(project.Tasks)

	// Save and backup
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	if err := BackupProject(project); err != nil {
		t.Fatalf("BackupProject failed: %v", err)
	}

	// Modify project - add a task
	newTask, err := models.NewTask("new-task", project.NextShortID, models.TaskTypeTask, "New Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, newTask)
	project.NextShortID++
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject after modification failed: %v", err)
	}

	// Restore from backup
	if err := RestoreFromBackup(project); err != nil {
		t.Fatalf("RestoreFromBackup failed: %v", err)
	}

	// Load and verify task count is back to original
	loadedProject := testutil.NewTestProject("Restore Test")
	loadedProject.Path = project.Path

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(loadedProject.Tasks) != originalTaskCount {
		t.Errorf("expected %d tasks after restore, got %d", originalTaskCount, len(loadedProject.Tasks))
	}
}

func TestRestoreFromBackup_Reversible(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Reversible Test")
	project.Path = filepath.Join(dir, "reversible-test")

	// Create initial state with 1 task
	task1, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task One")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*models.Task{task1}
	project.NextShortID = 2
	if err = SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	if err = BackupProject(project); err != nil {
		t.Fatalf("BackupProject failed: %v", err)
	}

	// Modify to 2 tasks
	task2, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Task Two")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task2)
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// First restore - should have 1 task
	if err := RestoreFromBackup(project); err != nil {
		t.Fatalf("RestoreFromBackup failed: %v", err)
	}

	loadedProject := testutil.NewTestProject("Reversible Test")
	loadedProject.Path = project.Path
	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if len(loadedProject.Tasks) != 1 {
		t.Errorf("expected 1 task after first restore, got %d", len(loadedProject.Tasks))
	}

	// Second restore (undo) - should have 2 tasks again
	if err := RestoreFromBackup(project); err != nil {
		t.Fatalf("second RestoreFromBackup failed: %v", err)
	}

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if len(loadedProject.Tasks) != 2 {
		t.Errorf("expected 2 tasks after second restore (undo), got %d", len(loadedProject.Tasks))
	}
}

func TestCreateProjectDir(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("New Project")
	project.Path = filepath.Join(dir, "new-project")

	if err := CreateProjectDir(project); err != nil {
		t.Fatalf("CreateProjectDir failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(project.Path); os.IsNotExist(err) {
		t.Error("project directory was not created")
	}

	// Verify tasks.md exists
	tasksPath := filepath.Join(project.Path, "tasks.md")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Error("tasks.md was not created")
	}

	// Verify ProjectExists returns true
	if !ProjectExists(project.Path) {
		t.Error("ProjectExists returned false but project exists")
	}
}

func TestDeleteProjectDir(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Delete Me")
	project.Path = filepath.Join(dir, "delete-me")

	if err := CreateProjectDir(project); err != nil {
		t.Fatalf("CreateProjectDir failed: %v", err)
	}

	if err := DeleteProjectDir(project.Path); err != nil {
		t.Fatalf("DeleteProjectDir failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(project.Path); !os.IsNotExist(err) {
		t.Error("project directory still exists after delete")
	}

	// Verify ProjectExists returns false
	if ProjectExists(project.Path) {
		t.Error("ProjectExists returned true but project was deleted")
	}
}

// TestSaveProject_FilePermissions verifies that SaveProject always writes the
// tasks file with FilePermissions (0644), even when the existing file has
// different permissions. The atomic write via a temp file + rename is what
// enforces this: permissions are set on the temp file before rename.
func TestSaveProject_FilePermissions(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Test Project")
	project.Path = filepath.Join(dir, "perm-test-project")

	// Create the project dir and a tasks.md with wrong permissions (0600).
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(project.Path, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("# Project: Test Project\n\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// SaveProject should rewrite the file atomically, enforcing FilePermissions.
	if err := SaveProject(project); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	info, err := os.Stat(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	got := info.Mode().Perm()
	want := os.FileMode(FilePermissions)
	if got != want {
		t.Errorf("file permissions = %04o, want %04o", got, want)
	}
}

// Helper to find a task by ID in a slice
func findTaskByID(tasks []*models.Task, id string) *models.Task {
	for _, task := range tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}
