package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
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
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
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
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
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
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{project}, results)

		if len(results.duplicateIDs) != 0 {
			t.Fatalf("expected 0 duplicate ID issues, got %d", len(results.duplicateIDs))
		}
	})
}

func TestCheckBrokenParentRefs(t *testing.T) {
	project := testutil.NewTestProject("P1")
	parent := testutil.NewTestTask("Parent", models.TaskTypeEpic)
	child := testutil.NewTestTask("Child", models.TaskTypeTask)
	orphan := testutil.NewTestTask("Orphan", models.TaskTypeTask)

	child.ParentID = &parent.ID
	badID := "nonexistent-uuid"
	orphan.ParentID = &badID

	project.Tasks = []*models.Task{parent, child, orphan}

	results := &doctorResults{}
	checkBrokenParentRefs([]*models.Project{project}, results)

	if len(results.brokenParentRefs) != 1 {
		t.Fatalf("expected 1 broken parent ref, got %d", len(results.brokenParentRefs))
	}
	if results.brokenParentRefs[0].task != orphan.ID {
		t.Errorf("expected task %q, got %q", orphan.ID, results.brokenParentRefs[0].task)
	}
}

func TestCheckDeadDependencies(t *testing.T) {
	project := testutil.NewTestProject("P1")
	task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
	task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
	task2.DependsOn = []string{task1.ID}        // valid
	task1.DependsOn = []string{"nonexistent-x"} // dead

	project.Tasks = []*models.Task{task1, task2}

	results := &doctorResults{}
	checkDeadDependencies([]*models.Project{project}, results)

	if len(results.deadDependencies) != 1 {
		t.Fatalf("expected 1 dead dependency, got %d", len(results.deadDependencies))
	}
	if results.deadDependencies[0].task != task1.ID {
		t.Errorf("expected task %q, got %q", task1.ID, results.deadDependencies[0].task)
	}
}

func TestCheckOrphanedCrossRefs(t *testing.T) {
	p1 := testutil.NewTestProject("P1")
	p2 := testutil.NewTestProject("P2")
	task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
	task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)

	// Valid cross-project ref
	task1.DependsOn = []string{p2.Slug + ":" + task2.ID}
	// Orphaned cross-project ref to nonexistent project
	task1.RelatedTo = []string{"nonexistent-project:some-task-id"}

	p1.Tasks = []*models.Task{task1}
	p2.Tasks = []*models.Task{task2}

	cfg := testutil.NewTestConfig()
	cfg.Projects = append(cfg.Projects, p1, p2)

	results := &doctorResults{}
	checkOrphanedCrossRefs([]*models.Project{p1, p2}, cfg, results)

	if len(results.orphanedCrossRefs) != 1 {
		t.Fatalf("expected 1 orphaned cross ref, got %d", len(results.orphanedCrossRefs))
	}
}

func TestTotalIssues(t *testing.T) {
	results := &doctorResults{
		orphanedCrossRefs: []doctorIssue{{project: "p"}},
		duplicateIDs:      []doctorIssue{{project: "p"}, {project: "p"}},
		emptyProjects:     []doctorIssue{{project: "p"}, {project: "p"}, {project: "p"}},
	}

	// emptyProjects should NOT be counted
	if got := results.totalIssues(); got != 3 {
		t.Errorf("totalIssues() = %d, want 3 (emptyProjects excluded)", got)
	}
}

func TestFixIssues(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "FixTest")
	defer deps.Cleanup()

	task1 := testutil.NewTestTask("Good Task", models.TaskTypeTask)
	task2 := testutil.NewTestTask("Broken Task", models.TaskTypeTask)
	badParent := "nonexistent-parent"
	task2.ParentID = &badParent
	task2.DependsOn = []string{"nonexistent-dep"}

	project.Tasks = []*models.Task{task1, task2}

	// Save initial state so fixIssues can persist
	deps.Repository.Projects[project.ID] = project

	cfg := testutil.NewTestConfig()

	fixIssues([]*models.Project{project}, cfg)

	// Verify broken parent ref was cleared
	if task2.ParentID != nil {
		t.Errorf("expected ParentID to be cleared, got %v", task2.ParentID)
	}

	// Verify dead dependency was removed
	if len(task2.DependsOn) != 0 {
		t.Errorf("expected DependsOn to be empty, got %v", task2.DependsOn)
	}
}

func TestReportBatchErrors(t *testing.T) {
	t.Run("no errors returns nil", func(t *testing.T) {
		err := ReportBatchErrors(3, nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("partial success returns nil", func(t *testing.T) {
		err := ReportBatchErrors(2, []string{"task 5: not found"})
		if err != nil {
			t.Errorf("expected nil for partial success, got %v", err)
		}
	})

	t.Run("total failure returns error", func(t *testing.T) {
		err := ReportBatchErrors(0, []string{"task 1: not found", "task 2: not found"})
		if err == nil {
			t.Error("expected error for total failure, got nil")
		}
	})
}
