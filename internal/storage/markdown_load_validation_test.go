package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
	"github.com/sglavoie/docket/internal/tterrors"
)

// TestLoadProjectRejectsBrokenHierarchyRatherThanRepairing pins why the load
// path carries no repair warnings to report. LoadProject parses before it
// builds the tree, and ParseProjectDocument already rejects a missing
// project-local parent, a parent cycle, and a depth overflow — the same three
// conditions BuildTaskTree would otherwise repair in memory. A corrupted file
// is therefore refused rather than silently rewritten in memory; doctor alone
// reads the missing parent through LoadProjectForRepair, to report and repair it.
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
	// The reader is told which file to open and which schema it must satisfy,
	// never a schema the chain retired long ago.
	wantFile := filepath.Join(project.Path, "tasks.md")
	wantSchema := fmt.Sprintf("schema-%d task document", CurrentTaskSchemaVersion)
	for _, want := range []string{wantFile, wantSchema} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("load error %q does not name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "schema-v6") {
		t.Errorf("load error %q names the retired schema-v6", err)
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

// TestLoadProjectForRepairKeepsTheRepairableDefects pins the doctor reader:
// the two defects doctor --fix repairs load as stored, so doctor can report
// them, while the strict reader keeps refusing the same bytes.
func TestLoadProjectForRepairKeepsTheRepairableDefects(t *testing.T) {
	project := danglingParentProject(t)
	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	duplicated := strings.Replace(string(content), "short_id: 2,", "short_id: 1,", 1)
	if duplicated == string(content) {
		t.Fatalf("could not duplicate the short ID in:\n%s", content)
	}
	if writeErr := os.WriteFile(tasksPath, []byte(duplicated), FilePermissions); writeErr != nil {
		t.Fatalf("write corrupted tasks file: %v", writeErr)
	}

	if strictErr := LoadProject(project); strictErr == nil {
		t.Fatal("the strict reader accepted a duplicate short ID and a dangling parent")
	}
	revision, err := LoadProjectForRepair(project)
	if err != nil {
		t.Fatalf("LoadProjectForRepair: %v", err)
	}
	if revision == nil || *revision != RawArtifactRevision([]byte(duplicated)) {
		t.Fatal("the repair reader did not return the raw revision of the bytes it read")
	}
	if len(project.Tasks) != 2 || project.Tasks[0].ShortID != 1 || project.Tasks[1].ShortID != 1 {
		t.Fatalf("tasks = %+v, want both tasks with the stored duplicate short ID", project.Tasks)
	}
	child := project.Tasks[1]
	if child.ParentID == nil || *child.ParentID != "parent-1" || child.Parent != nil {
		t.Fatalf("child parent = %v, want the stored missing reference kept on a root", child.ParentID)
	}
	if project.NextShortID != 2 {
		t.Fatalf("NextShortID = %d, want 2", project.NextShortID)
	}
}

// TestLoadProjectForRepairRefusesEveryOtherDefect pins that the repair reader
// widens nothing beyond the two repairable defects: a duplicate task identity
// still fails with the strict reader's error.
func TestLoadProjectForRepairRefusesEveryOtherDefect(t *testing.T) {
	project := danglingParentProject(t)
	tasksPath := filepath.Join(project.Path, "tasks.md")
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	corrupted := strings.Replace(string(content), "id: child-1,", "id: renamed-1,", 1)
	if corrupted == string(content) {
		t.Fatalf("could not rename the child task in:\n%s", content)
	}
	if writeErr := os.WriteFile(tasksPath, []byte(corrupted), FilePermissions); writeErr != nil {
		t.Fatalf("write corrupted tasks file: %v", writeErr)
	}

	strictErr := LoadProject(project)
	if strictErr == nil {
		t.Fatal("the strict reader accepted a duplicate task identity")
	}
	if _, err := LoadProjectForRepair(project); err == nil || err.Error() != strictErr.Error() {
		t.Fatalf("LoadProjectForRepair error = %v, want the strict error %v", err, strictErr)
	}
}
