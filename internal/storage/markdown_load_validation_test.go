package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
)

// TestLoadProjectRejectsBrokenHierarchyRatherThanRepairing pins why the load
// path carries no repair warnings to report. LoadProject parses before it
// builds the tree, and ParseProjectDocument already rejects a missing
// project-local parent, a parent cycle, and a depth overflow — the same three
// conditions BuildTaskTree would otherwise repair in memory. A corrupted file
// is therefore a counted load failure that doctor prints under "Project load
// failures", not a silent in-memory rewrite.
func TestLoadProjectRejectsBrokenHierarchyRatherThanRepairing(t *testing.T) {
	project := danglingParentProject(t)

	err := LoadProject(project)
	if err == nil {
		t.Fatal("dangling parent loaded successfully; want a parse error")
	}
	if !errors.Is(err, tterrors.ErrParseError) {
		t.Fatalf("load error %v is not an ErrParseError", err)
	}
	if !strings.Contains(err.Error(), "references missing project-local parent") {
		t.Fatalf("load error %q does not name the broken reference", err)
	}
}

// TestLoadMissingProjectIsAnEmptyLoad pins the absent-file path as a success
// rather than an error, since a project directory may legitimately predate its
// first task.
func TestLoadMissingProjectIsAnEmptyLoad(t *testing.T) {
	dir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	project := testutil.NewTestProject("Absent")
	project.Path = filepath.Join(dir, "absent")

	if err := LoadProject(project); err != nil {
		t.Fatalf("LoadProject on a missing tasks file failed: %v", err)
	}
	if len(project.Tasks) != 0 {
		t.Fatalf("missing tasks file produced %d tasks", len(project.Tasks))
	}
	if project.NextShortID != 1 {
		t.Fatalf("NextShortID = %d, want 1", project.NextShortID)
	}
}

// danglingParentProject writes a tasks.md whose child names a parent ID no task
// in the file carries, by renaming the parent after a valid save.
func danglingParentProject(t *testing.T) *models.Project {
	t.Helper()
	dir, cleanup := testutil.TempProjectDir(t)
	t.Cleanup(cleanup)

	project := testutil.NewTestProject("Dangling")
	project.Path = filepath.Join(dir, "dangling")
	parent, err := models.NewGenericTask("parent-1", 1, "Parent task")
	if err != nil {
		t.Fatal(err)
	}
	child, err := models.NewGenericTask("child-1", 2, "Child task")
	if err != nil {
		t.Fatal(err)
	}
	parentID := parent.ID
	child.ParentID = &parentID
	project.Tasks = []*models.Task{parent, child}
	project.NextShortID = 3
	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("SaveProject failed: %v", saveErr)
	}

	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	corrupted := strings.Replace(string(content), "id: parent-1,", "id: renamed-1,", 1)
	if corrupted == string(content) {
		t.Fatalf("could not rename the parent task in:\n%s", content)
	}
	if writeErr := os.WriteFile(tasksPath, []byte(corrupted), FilePermissions); writeErr != nil {
		t.Fatalf("write corrupted tasks file: %v", writeErr)
	}

	reloaded := testutil.NewTestProject("Dangling")
	reloaded.Path = project.Path
	return reloaded
}
