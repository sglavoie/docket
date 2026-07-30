package storage

import (
	"os"
	"path/filepath"
	"strings"
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
	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject failed: %v", saveErr)
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
		if task.State() != original.State() {
			t.Errorf("task %s: expected state %s, got %s", task.ID, original.State(), task.State())
		}
		if task.Priority != original.Priority {
			t.Errorf("task %s: expected priority %d, got %d", task.ID, original.Priority, task.Priority)
		}
	}
}

func TestSaveProject_OmitsLegacyProjectSections(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Sections Test")
	project.Path = filepath.Join(dir, "sections-test")

	task, err := models.NewGenericTask("task-1", 1, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(models.StateInbox)
	project.Tasks = []*models.Task{task}

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject failed: %v", saveErr)
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read saved project: %v", err)
	}
	if !strings.Contains(string(content), "<!-- tt-schema: 5 -->") {
		t.Fatalf("saved project is missing schema-v5 marker:\n%s", content)
	}
	if strings.Contains(string(content), "<!-- sections:") {
		t.Fatalf("saved project retained legacy section metadata:\n%s", content)
	}

	// Load and verify
	loadedProject := testutil.NewTestProject("Sections Test")
	loadedProject.Path = project.Path

	if err := LoadProject(loadedProject); err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(loadedProject.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(loadedProject.Tasks))
	}

	if got := loadedProject.Tasks[0].State(); got != models.StateInbox {
		t.Errorf("expected task section Inbox, got %s", got)
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

	// Verify projectExists returns true
	if !projectExists(project.Path) {
		t.Error("projectExists returned false but project exists")
	}
}

func Test_deleteProjectDir(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Delete Me")
	project.Path = filepath.Join(dir, "delete-me")

	if err := CreateProjectDir(project); err != nil {
		t.Fatalf("CreateProjectDir failed: %v", err)
	}

	if err := deleteProjectDir(project.Path); err != nil {
		t.Fatalf("deleteProjectDir failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(project.Path); !os.IsNotExist(err) {
		t.Error("project directory still exists after delete")
	}

	// Verify projectExists returns false
	if projectExists(project.Path) {
		t.Error("projectExists returned true but project was deleted")
	}
}

// TestSaveProject_FilePermissions verifies that SaveProject tightens a
// world-readable tasks file down to the private FilePermissions (0600) default.
// The atomic write via a temp file + rename is what enforces this: permissions
// are set on the temp file before rename, and a looser existing mode is
// narrowed rather than preserved.
func TestSaveProject_FilePermissions(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Test Project")
	project.Path = filepath.Join(dir, "perm-test-project")

	// Create the project dir and a tasks.md with a looser mode (0644) than the
	// private default so we can assert it gets tightened.
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(project.Path, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("# Project: Test Project\n\n"), 0644); err != nil {
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

// TestLockFileName verifies the lock file name is a hidden file (dot-prefixed)
// and matches the expected value. This prevents regressions where callers use
// different lock paths (which was a bug: cmd/ used "tasks.md.lock" while
// storage/ used ".tasks.md.lock", so the locks didn't protect each other).
func TestLockFileName(t *testing.T) {
	if LockFileName != ".tasks.md.lock" {
		t.Errorf("LockFileName = %q, want %q", LockFileName, ".tasks.md.lock")
	}
}

// TestSaveProjectLocked_CreatesLockFile verifies that SaveProjectLocked uses
// the correct lock file path (the one from LockFileName constant).
func TestSaveProjectLocked_CreatesLockFile(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Lock Test")
	project.Path = filepath.Join(dir, "lock-test")
	project.Tasks = []*models.Task{}

	if err := SaveProjectLocked(project); err != nil {
		t.Fatalf("SaveProjectLocked failed: %v", err)
	}

	// The lock file should exist after save (flock creates it)
	lockPath := filepath.Join(project.Path, LockFileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Errorf("lock file %q not created by SaveProjectLocked", LockFileName)
	}
}

func TestWriteTasksMarkdown_SkipsNilTasks(t *testing.T) {
	tk1, err := models.NewGenericTask("task-1", 1, "First")
	if err != nil {
		t.Fatal(err)
	}
	tk2, err := models.NewGenericTask("task-2", 2, "Second")
	if err != nil {
		t.Fatal(err)
	}

	// A nil entry must not crash the writer. Real-world callers reach this
	// path via undo_targeted and other code paths that bypass the audit and
	// may carry transient nil entries during reconstruction.
	tasks := []*models.Task{tk1, nil, tk2}

	out := WriteTasksMarkdown("Nil Test", tasks)

	parsed, err := ParseTasksMarkdownDocument(out)
	if err != nil {
		t.Fatalf("ParseTasksMarkdownDocument failed: %v", err)
	}
	if len(parsed.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after round-trip, got %d", len(parsed.Tasks))
	}
	if parsed.Tasks[0].Title != "First" || parsed.Tasks[1].Title != "Second" {
		t.Errorf("unexpected titles after round-trip: %q, %q", parsed.Tasks[0].Title, parsed.Tasks[1].Title)
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
