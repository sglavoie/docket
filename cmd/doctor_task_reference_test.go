package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestCheckTaskReferencesReportsEveryDefectShape covers the five conditions the
// scan calls broken. Each is a reference no mutation could have produced, so
// each is reported for a human to repair rather than pruned.
func TestCheckTaskReferencesReportsEveryDefectShape(t *testing.T) {
	holder := testutil.NewTestProject("Holder")
	target := testutil.NewTestProject("Target")
	present := testutil.NewTestTask("Present")
	target.Tasks = []*models.Task{present}

	malformed := testutil.NewTestTask("Malformed")
	malformed.Dependencies = []models.TaskReference{{ProjectID: "", TaskID: "t-1"}}
	duplicated := testutil.NewTestTask("Duplicated")
	duplicated.Dependencies = []models.TaskReference{
		{ProjectID: target.ID, TaskID: present.ID},
		{ProjectID: target.ID, TaskID: present.ID},
	}
	selfRef := testutil.NewTestTask("Self")
	selfRef.Dependencies = []models.TaskReference{{ProjectID: holder.ID, TaskID: selfRef.ID}}
	unknownProject := testutil.NewTestTask("Unknown project")
	unknownProject.Dependencies = []models.TaskReference{{ProjectID: "proj-gone", TaskID: "t-gone"}}
	unknownTask := testutil.NewTestTask("Unknown task")
	unknownTask.Dependencies = []models.TaskReference{{ProjectID: target.ID, TaskID: "t-never-existed"}}
	holder.Tasks = []*models.Task{malformed, duplicated, selfRef, unknownProject, unknownTask}

	every := []*models.Project{holder, target}
	results := &doctorResults{}
	checkTaskReferences(every, every, results)

	if len(results.unavailableTaskRefs) != 0 {
		t.Fatalf("unavailable notes = %+v, want none", results.unavailableTaskRefs)
	}
	if len(results.brokenTaskRefs) != 5 {
		t.Fatalf("broken references = %d (%+v), want 5", len(results.brokenTaskRefs), results.brokenTaskRefs)
	}
	for _, want := range []string{
		"malformed reference",
		"duplicate reference",
		"references itself",
		"names an unknown project",
		"names a task its project does not own",
	} {
		found := false
		for _, issue := range results.brokenTaskRefs {
			if strings.Contains(issue.detail, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no broken reference reported %q: %+v", want, results.brokenTaskRefs)
		}
	}
}

// TestCheckTaskReferencesTreatsAnUnloadedProjectAsUnavailable locks the
// distinction the integrity contract turns on: a project the catalog knows but
// this run did not load says nothing about whether the target exists, so the
// reference is an informational note and never counts as an issue.
func TestCheckTaskReferencesTreatsAnUnloadedProjectAsUnavailable(t *testing.T) {
	holder := testutil.NewTestProject("Holder")
	unloaded := testutil.NewTestProject("Unloaded")
	task := testutil.NewTestTask("Holder task")
	task.Dependencies = []models.TaskReference{{ProjectID: unloaded.ID, TaskID: "t-elsewhere"}}
	holder.Tasks = []*models.Task{task}

	results := &doctorResults{}
	checkTaskReferences([]*models.Project{holder, unloaded}, []*models.Project{holder}, results)

	if len(results.brokenTaskRefs) != 0 {
		t.Fatalf("broken references = %+v, want none", results.brokenTaskRefs)
	}
	if len(results.unavailableTaskRefs) != 1 ||
		!strings.Contains(results.unavailableTaskRefs[0].detail, "its project did not load") {
		t.Fatalf("unavailable notes = %+v", results.unavailableTaskRefs)
	}
	if results.totalIssues() != 0 {
		t.Fatalf("totalIssues = %d, want 0: an unavailable reference is not an issue", results.totalIssues())
	}
}

// TestCheckTaskReferencesNeverPrunes proves the scan is read-only: a reported
// reference is still stored on the task afterwards, because repairing external
// corruption by deleting the evidence is exactly what this check exists to
// prevent.
func TestCheckTaskReferencesNeverPrunes(t *testing.T) {
	holder := testutil.NewTestProject("Holder")
	task := testutil.NewTestTask("Holder task")
	task.Dependencies = []models.TaskReference{{ProjectID: "proj-gone", TaskID: "t-gone"}}
	holder.Tasks = []*models.Task{task}

	results := &doctorResults{}
	projects := []*models.Project{holder}
	checkTaskReferences(projects, projects, results)
	fixed, err := fixIssues(t.TempDir(), projects)
	if err != nil || len(fixed) != 0 {
		t.Fatalf("fixIssues = %v, %v; want nothing fixed and no error", fixed, err)
	}

	if len(task.Dependencies) != 1 {
		t.Fatalf("task references after doctor = %v, want the corrupted reference retained", task.Dependencies)
	}
	if len(results.brokenTaskRefs) != 1 {
		t.Fatalf("broken references = %+v, want exactly one", results.brokenTaskRefs)
	}
	if results.fixableIssues() != 0 {
		t.Fatalf("fixableIssues = %d, want 0: a broken reference is never auto-repaired", results.fixableIssues())
	}
}
