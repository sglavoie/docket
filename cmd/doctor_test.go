package cmd

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCheckDuplicateShortIDs(t *testing.T) {
	t.Run("duplicate within project", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
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
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
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
		task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
		task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
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

func TestCheckDeadRelatedTo(t *testing.T) {
	project := testutil.NewTestProject("P1")
	task1 := testutil.NewTestTask("Task A", models.TaskTypeTask)
	task2 := testutil.NewTestTask("Task B", models.TaskTypeTask)
	task1.RelatedTo = []string{task2.ID}            // valid
	task2.RelatedTo = []string{"nonexistent-rel-x"} // dead

	project.Tasks = []*models.Task{task1, task2}

	results := &doctorResults{}
	checkDeadDependencies([]*models.Project{project}, results)

	if len(results.deadDependencies) != 1 {
		t.Fatalf("expected 1 dead RelatedTo ref, got %d", len(results.deadDependencies))
	}
	if results.deadDependencies[0].task != task2.ID {
		t.Errorf("expected task %q, got %q", task2.ID, results.deadDependencies[0].task)
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

func TestCheckOrphanedCrossRefs_ArchivedProjectNotOrphaned(t *testing.T) {
	// Cross-project references to tasks in archived projects should NOT be
	// reported as orphaned. The doctor command must load archived projects
	// for cross-ref validation.
	activeProject := testutil.NewTestProject("Active")
	archivedProject := testutil.NewTestProject("Archived")
	archivedProject.Archived = true

	activeTask := testutil.NewTestTask("Active Task", models.TaskTypeTask)
	archivedTask := testutil.NewTestTask("Archived Task", models.TaskTypeTask)

	// Active project references a task in the archived project
	activeTask.DependsOn = []string{archivedProject.Slug + ":" + archivedTask.ID}

	activeProject.Tasks = []*models.Task{activeTask}
	archivedProject.Tasks = []*models.Task{archivedTask}

	cfg := testutil.NewTestConfig()
	cfg.Projects = append(cfg.Projects, activeProject, archivedProject)

	results := &doctorResults{}
	// Pass BOTH projects (including archived) to simulate the fix
	checkOrphanedCrossRefs([]*models.Project{activeProject, archivedProject}, cfg, results)

	if len(results.orphanedCrossRefs) != 0 {
		t.Errorf("expected 0 orphaned cross refs (archived project ref is valid), got %d: %v",
			len(results.orphanedCrossRefs), results.orphanedCrossRefs)
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

func TestFixIssues_CleansDeadRelatedTo(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "FixRelated")
	defer deps.Cleanup()

	task1 := testutil.NewTestTask("Good Task", models.TaskTypeTask)
	task2 := testutil.NewTestTask("Broken Related", models.TaskTypeTask)
	task2.RelatedTo = []string{task1.ID, "nonexistent-rel"}

	project.Tasks = []*models.Task{task1, task2}
	deps.Repository.Projects[project.ID] = project

	cfg := testutil.NewTestConfig()
	fixIssues([]*models.Project{project}, cfg)

	if len(task2.RelatedTo) != 1 {
		t.Errorf("expected 1 RelatedTo ref after fix, got %d: %v", len(task2.RelatedTo), task2.RelatedTo)
	}
	if len(task2.RelatedTo) > 0 && task2.RelatedTo[0] != task1.ID {
		t.Errorf("expected RelatedTo[0] to be %q, got %q", task1.ID, task2.RelatedTo[0])
	}
}

func TestCheckMissingDescriptionMarkers(t *testing.T) {
	t.Run("flags project with old-format descriptions", func(t *testing.T) {
		dir := t.TempDir()
		project := testutil.NewTestProject("OldFormat")
		project.Path = dir

		task := testutil.NewTestTask("My Task", models.TaskTypeTask)
		task.Description = "Some description"
		project.Tasks = []*models.Task{task}

		// Old-format file: description inline, no DESCRIPTION markers
		content := "## Task: My Task\n<!-- id: " + task.ID + " -->\n\nSome description\n"
		if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write tasks.md: %v", err)
		}

		results := &doctorResults{}
		checkMissingDescriptionMarkers([]*models.Project{project}, results)

		if len(results.missingDescMarkers) != 1 {
			t.Fatalf("expected 1 issue, got %d", len(results.missingDescMarkers))
		}
		if results.missingDescMarkers[0].project != "OldFormat" {
			t.Errorf("expected project 'OldFormat', got %q", results.missingDescMarkers[0].project)
		}
	})

	t.Run("no issue when tasks have no descriptions", func(t *testing.T) {
		dir := t.TempDir()
		project := testutil.NewTestProject("NoDesc")
		project.Path = dir

		task := testutil.NewTestTask("My Task", models.TaskTypeTask)
		project.Tasks = []*models.Task{task}

		content := "## Task: My Task\n<!-- id: " + task.ID + " -->\n"
		if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write tasks.md: %v", err)
		}

		results := &doctorResults{}
		checkMissingDescriptionMarkers([]*models.Project{project}, results)

		if len(results.missingDescMarkers) != 0 {
			t.Fatalf("expected 0 issues, got %d", len(results.missingDescMarkers))
		}
	})
}

func TestCheckMissingDescriptionMarkers_NoFalsePositive_NewFormat(t *testing.T) {
	dir := t.TempDir()
	project := testutil.NewTestProject("NewFormat")
	project.Path = dir

	task := testutil.NewTestTask("My Task", models.TaskTypeTask)
	task.Description = "Some description"
	project.Tasks = []*models.Task{task}

	// New-format file: description already wrapped in DESCRIPTION markers
	content := "## Task: My Task\n<!-- id: " + task.ID + " -->\n\n<!-- DESCRIPTION -->\nSome description\n<!-- /DESCRIPTION -->\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	results := &doctorResults{}
	checkMissingDescriptionMarkers([]*models.Project{project}, results)

	if len(results.missingDescMarkers) != 0 {
		t.Fatalf("expected 0 issues (already new format), got %d", len(results.missingDescMarkers))
	}
}

func TestDoctor_Fix_MigratesDescriptionMarkers(t *testing.T) {
	dir := t.TempDir()
	project := testutil.NewTestProject("MigrateTest")
	project.Path = dir

	task := testutil.NewTestTask("Task With Desc", models.TaskTypeTask)
	task.ShortID = 1
	task.Description = "Some description"
	project.Tasks = []*models.Task{task}

	// Write old-format file
	content := "## Task: Task With Desc\n<!-- id: " + task.ID + " -->\n\nSome description\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	// Check before fix — should be flagged
	results := &doctorResults{}
	checkMissingDescriptionMarkers([]*models.Project{project}, results)
	if len(results.missingDescMarkers) != 1 {
		t.Fatalf("expected 1 issue before fix, got %d", len(results.missingDescMarkers))
	}

	// Fix — SaveProjectLocked re-writes tasks.md with DESCRIPTION markers
	fixMissingDescriptionMarkers(results.missingDescMarkers, []*models.Project{project})

	// Check after fix — should be clean
	results2 := &doctorResults{}
	checkMissingDescriptionMarkers([]*models.Project{project}, results2)
	if len(results2.missingDescMarkers) != 0 {
		t.Fatalf("expected 0 issues after fix, got %d", len(results2.missingDescMarkers))
	}

	// Verify the written file contains DESCRIPTION markers
	raw, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
	if err != nil {
		t.Fatalf("failed to read tasks.md: %v", err)
	}
	if !strings.Contains(string(raw), "<!-- DESCRIPTION -->") {
		t.Error("expected DESCRIPTION markers in tasks.md after fix")
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
