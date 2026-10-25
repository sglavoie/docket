package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
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
	if !strings.Contains(string(content), schemaMarkerV20) {
		t.Fatalf("saved project is missing current schema marker:\n%s", content)
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

func TestWriteProjectMarkdownRejectsNilTasks(t *testing.T) {
	tk1, err := models.NewGenericTask("task-1", 1, "First")
	if err != nil {
		t.Fatal(err)
	}
	tk2, err := models.NewGenericTask("task-2", 2, "Second")
	if err != nil {
		t.Fatal(err)
	}

	tasks := []*models.Task{tk1, nil, tk2}

	if _, err := WriteProjectMarkdown(&models.Project{Name: "Nil Test", Tasks: tasks}); err == nil || !strings.Contains(err.Error(), "nil task") {
		t.Fatalf("WriteProjectMarkdown error = %v, want nil-task rejection", err)
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

// TestSaveProject_RefusesEmptyPath pins the filesystem refusal: a project rooted
// at no directory would otherwise write tasks.md and its lock beneath the
// working directory.
func TestSaveProject_RefusesEmptyPath(t *testing.T) {
	t.Chdir(t.TempDir())
	project := testutil.NewTestProject("Unrooted")
	project.Tasks = []*models.Task{}

	for name, save := range map[string]func(*models.Project) error{
		"SaveProject": SaveProject,
	} {
		if err := save(project); !errors.Is(err, ErrNoProjectPath) {
			t.Errorf("%s(Path \"\") = %v, want ErrNoProjectPath", name, err)
		}
	}
	for _, name := range []string{"tasks.md", LockFileName} {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			t.Errorf("refused save left %s in the working directory", name)
		}
	}
}
