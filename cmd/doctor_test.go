package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
)

func TestCheckMissingTaskFiles(t *testing.T) {
	// Project with a nonexistent path → should report missing task file
	project := testutil.NewTestProject("Ghost")
	project.Path = t.TempDir() // exists but no tasks.md inside

	results := &doctorResults{}
	checkMissingTaskFiles([]*models.Project{project}, results)

	if len(results.missingTaskFiles) != 1 {
		t.Fatalf("expected 1 missing task file, got %d", len(results.missingTaskFiles))
	}
	if results.missingTaskFiles[0].project != "Ghost" {
		t.Errorf("expected project 'Ghost', got %q", results.missingTaskFiles[0].project)
	}
}

func TestCheckEmptyProjects(t *testing.T) {
	empty := testutil.NewTestProject("Empty")
	nonEmpty := testutil.NewTestProjectWithTasks("Has Tasks")

	results := &doctorResults{}
	checkEmptyProjects([]*models.Project{empty, nonEmpty}, results)

	if len(results.emptyProjects) != 1 {
		t.Fatalf("expected 1 empty project, got %d", len(results.emptyProjects))
	}
	if results.emptyProjects[0].project != "Empty" {
		t.Errorf("expected project 'Empty', got %q", results.emptyProjects[0].project)
	}
}

func TestCheckDuplicateIDs(t *testing.T) {
	t.Run("duplicate within project", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task2.ID = task1.ID // force duplicate
		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{project}, results)

		if len(results.duplicateIDs) == 0 {
			t.Fatal("expected at least 1 duplicate ID issue, got 0")
		}
	})

	t.Run("duplicate across projects", func(t *testing.T) {
		p1 := testutil.NewTestProject("P1")
		p2 := testutil.NewTestProject("P2")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task2.ID = task1.ID // force same ID across projects
		p1.Tasks = []*models.Task{task1}
		p2.Tasks = []*models.Task{task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{p1, p2}, results)

		if len(results.duplicateIDs) != 1 {
			t.Fatalf("expected 1 cross-project duplicate, got %d", len(results.duplicateIDs))
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{project}, results)

		if len(results.duplicateIDs) != 0 {
			t.Fatalf("expected 0 duplicate ID issues, got %d", len(results.duplicateIDs))
		}
	})
}

func TestCheckDuplicateShortIDs(t *testing.T) {
	t.Run("duplicate within project", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 5
		task2.ShortID = 5 // force duplicate

		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{project}, results)

		if len(results.duplicateShortIDs) != 1 {
			t.Fatalf("expected 1 duplicate short ID issue, got %d", len(results.duplicateShortIDs))
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 1
		task2.ShortID = 2

		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{project}, results)

		if len(results.duplicateShortIDs) != 0 {
			t.Fatalf("expected 0 duplicate short ID issues, got %d", len(results.duplicateShortIDs))
		}
	})

	t.Run("duplicates across projects not flagged", func(t *testing.T) {
		p1 := testutil.NewTestProject("P1")
		p2 := testutil.NewTestProject("P2")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 1
		task2.ShortID = 1 // same short ID but different projects — OK

		p1.Tasks = []*models.Task{task1}
		p2.Tasks = []*models.Task{task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{p1, p2}, results)

		if len(results.duplicateShortIDs) != 0 {
			t.Fatalf("expected 0 issues (different projects), got %d", len(results.duplicateShortIDs))
		}
	})
}

func TestCheckBrokenParentRefs(t *testing.T) {
	project := testutil.NewTestProject("P1")
	parent := testutil.NewTestTask("Parent")
	child := testutil.NewTestTask("Child")
	orphan := testutil.NewTestTask("Orphan")

	child.ParentID = &parent.ID
	badID := "nonexistent-uuid"
	orphan.ParentID = &badID

	project.Tasks = []*models.Task{parent, child, orphan}

	results := &doctorResults{}
	checkBrokenParentRefs([]*models.Project{project}, results)

	if len(results.brokenParentRefs) != 1 {
		t.Fatalf("expected 1 broken parent ref, got %d", len(results.brokenParentRefs))
	}
	if !strings.Contains(results.brokenParentRefs[0].detail, orphan.Title) {
		t.Errorf("expected orphan detail, got %q", results.brokenParentRefs[0].detail)
	}
}

func TestCheckCompletionTimestamps(t *testing.T) {
	validCompleted := time.Now().AddDate(0, 0, -30)
	zero := time.Time{}
	project := testutil.NewTestProject("History")
	valid := testutil.NewTestTask("Valid terminal")
	valid.WorkflowState = models.StateDone
	valid.CompletedAt = &validCompleted
	missing := testutil.NewTestTask("Missing terminal timestamp")
	missing.WorkflowState = models.StateCanceled
	missing.CompletedAt = nil
	zeroTerminal := testutil.NewTestTask("Zero terminal timestamp")
	zeroTerminal.WorkflowState = models.StateDone
	zeroTerminal.CompletedAt = &zero
	stale := testutil.NewTestTask("Stale nonterminal timestamp")
	stale.WorkflowState = models.StateBacklog
	stale.CompletedAt = &validCompleted
	project.Tasks = []*models.Task{valid, missing, zeroTerminal, stale}

	results := &doctorResults{}
	checkCompletionTimestamps([]*models.Project{project}, results)

	if got := len(results.invalidCompletionTimestamps); got != 3 {
		t.Fatalf("invalid completion timestamps = %d, want 3: %+v", got, results.invalidCompletionTimestamps)
	}
}

func TestTotalIssues(t *testing.T) {
	results := &doctorResults{
		duplicateIDs:  []doctorIssue{{project: "p"}, {project: "p"}},
		emptyProjects: []doctorIssue{{project: "p"}, {project: "p"}, {project: "p"}},
	}

	// Empty projects are informational and not live schema-6 issues.
	if got := results.totalIssues(); got != 2 {
		t.Errorf("totalIssues() = %d, want 2", got)
	}
}
