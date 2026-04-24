package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestProcessDeferredMoves_WarnsCrossProjectDependencies(t *testing.T) {
	t.Run("warns when task has DependsOn being dropped", func(t *testing.T) {
		tmpDir := t.TempDir()
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		// Set up source project with a task that has a dependency
		srcProject := testutil.NewTestProject("Source")
		srcProject.Path = tmpDir + "/source"
		if err := os.MkdirAll(srcProject.Path, 0o755); err != nil {
			t.Fatal(err)
		}

		task, err := models.NewTask("task-1", srcProject.GetNextShortID(), models.TaskTypeTask, "My Task")
		if err != nil {
			t.Fatal(err)
		}
		task.DependsOn = []string{"dep-id-1", "dep-id-2"}
		srcProject.Tasks = append(srcProject.Tasks, task)

		// Set up destination project
		destProject := testutil.NewTestProject("Dest")
		destProject.Path = tmpDir + "/dest"
		if mkdirErr := os.MkdirAll(destProject.Path, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		// Wire up mock storage
		deps.Repository.Projects[srcProject.ID] = srcProject
		deps.Repository.Projects[destProject.ID] = destProject
		deps.Config.Projects = []*models.Project{srcProject, destProject}

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldRepo }()

		moves := []workflow.DeferredMove{
			{TaskID: task.ID, Destination: destProject.Slug},
		}

		// Capture stdout to verify warnings
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = processDeferredMoves(srcProject, moves, deps.Config, nil)

		_ = w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)

		testutil.AssertNoError(t, err)
		output := buf.String()
		if output == "" {
			t.Error("expected warning about dropped dependencies, got none")
		}
		if len(task.DependsOn) != 0 {
			t.Errorf("expected DependsOn to be cleared, got %v", task.DependsOn)
		}
	})

	t.Run("warns when task has RelatedTo being dropped", func(t *testing.T) {
		tmpDir := t.TempDir()
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		srcProject := testutil.NewTestProject("Source")
		srcProject.Path = tmpDir + "/source"
		if err := os.MkdirAll(srcProject.Path, 0o755); err != nil {
			t.Fatal(err)
		}

		task, err := models.NewTask("task-2", srcProject.GetNextShortID(), models.TaskTypeTask, "My Task")
		if err != nil {
			t.Fatal(err)
		}
		task.RelatedTo = []string{"rel-id-1"}
		srcProject.Tasks = append(srcProject.Tasks, task)

		destProject := testutil.NewTestProject("Dest")
		destProject.Path = tmpDir + "/dest"
		if mkdirErr := os.MkdirAll(destProject.Path, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		deps.Repository.Projects[srcProject.ID] = srcProject
		deps.Repository.Projects[destProject.ID] = destProject
		deps.Config.Projects = []*models.Project{srcProject, destProject}

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldRepo }()

		moves := []workflow.DeferredMove{
			{TaskID: task.ID, Destination: destProject.Slug},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = processDeferredMoves(srcProject, moves, deps.Config, nil)

		_ = w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)

		testutil.AssertNoError(t, err)
		output := buf.String()
		if output == "" {
			t.Error("expected warning about dropped relations, got none")
		}
		if len(task.RelatedTo) != 0 {
			t.Errorf("expected RelatedTo to be cleared, got %v", task.RelatedTo)
		}
	})

	t.Run("moves task with children to destination", func(t *testing.T) {
		tmpDir := t.TempDir()
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		srcProject := testutil.NewTestProject("Source")
		srcProject.Path = tmpDir + "/source"
		if err := os.MkdirAll(srcProject.Path, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create parent (epic) and child (task) hierarchy
		epic, err := models.NewTask("epic-1", srcProject.GetNextShortID(), models.TaskTypeEpic, "My Epic")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask("child-1", srcProject.GetNextShortID(), models.TaskTypeTask, "Child Task")
		if err != nil {
			t.Fatal(err)
		}
		child.ParentID = &epic.ID
		srcProject.Tasks = []*models.Task{epic, child}
		srcProject.BuildTaskTree()

		destProject := testutil.NewTestProject("Dest")
		destProject.Path = tmpDir + "/dest"
		if mkdirErr := os.MkdirAll(destProject.Path, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		deps.Repository.Projects[srcProject.ID] = srcProject
		deps.Repository.Projects[destProject.ID] = destProject
		deps.Config.Projects = []*models.Project{srcProject, destProject}

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldRepo }()

		moves := []workflow.DeferredMove{
			{TaskID: epic.ID, Destination: destProject.Slug},
		}

		// Suppress stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		err = processDeferredMoves(srcProject, moves, deps.Config, nil)

		_ = w.Close()
		os.Stdout = oldStdout

		testutil.AssertNoError(t, err)

		// Source project should have no tasks left
		if len(srcProject.Tasks) != 0 {
			t.Errorf("expected source to have 0 tasks, got %d", len(srcProject.Tasks))
		}

		// Destination should have both parent and child
		if len(destProject.Tasks) != 2 {
			t.Errorf("expected destination to have 2 tasks, got %d", len(destProject.Tasks))
		}

		// Verify the child is present in destination
		foundChild := false
		for _, task := range destProject.Tasks {
			if task.ID == "child-1" {
				foundChild = true
			}
		}
		if !foundChild {
			t.Error("expected child task to be moved to destination project")
		}
	})

	t.Run("no warning when task has no cross-project references", func(t *testing.T) {
		tmpDir := t.TempDir()
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		srcProject := testutil.NewTestProject("Source")
		srcProject.Path = tmpDir + "/source"
		if err := os.MkdirAll(srcProject.Path, 0o755); err != nil {
			t.Fatal(err)
		}

		task, err := models.NewTask("task-3", srcProject.GetNextShortID(), models.TaskTypeTask, "Clean Task")
		if err != nil {
			t.Fatal(err)
		}
		srcProject.Tasks = append(srcProject.Tasks, task)

		destProject := testutil.NewTestProject("Dest")
		destProject.Path = tmpDir + "/dest"
		if mkdirErr := os.MkdirAll(destProject.Path, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}

		deps.Repository.Projects[srcProject.ID] = srcProject
		deps.Repository.Projects[destProject.ID] = destProject
		deps.Config.Projects = []*models.Project{srcProject, destProject}

		oldRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldRepo }()

		moves := []workflow.DeferredMove{
			{TaskID: task.ID, Destination: destProject.Slug},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = processDeferredMoves(srcProject, moves, deps.Config, nil)

		_ = w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)

		testutil.AssertNoError(t, err)
		output := buf.String()
		if output != "" {
			t.Errorf("expected no warning for task without cross-project references, got: %q", output)
		}
	})
}

// TestSaveProject_ProjectLevelRulesEvaluated verifies that project-level
// workflow rules (e.g., set_project_status based on completion_ratio) are
// evaluated during SaveProject, not just in the TUI or `workflow eval`.
func TestSaveProject_ProjectLevelRulesEvaluated(t *testing.T) {
	deps := testutil.SetupTestDeps(t)
	defer deps.Cleanup()

	project := testutil.NewTestProject("ProjRules")
	project.Path = t.TempDir()

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	defer func() { storage.DefaultRepository = oldRepo }()
	deps.Repository.Projects[project.ID] = project

	// Create two tasks, both completed
	task1, err := models.NewTask("proj-t1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Status = models.StatusDone

	task2, err := models.NewTask("proj-t2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Status = models.StatusDone
	project.Tasks = []*models.Task{task1, task2}
	project.BuildTaskTree()

	// Add a project-level rule: when completion_ratio == 100, set project status
	cfg := deps.Config
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:      "proj-rule-1",
			Name:    "All Done",
			Level:   workflow.RuleLevelProject,
			Enabled: true,
			Triggers: []workflow.RuleTrigger{
				{
					Field:     workflow.FieldCompletionRatio,
					Operation: workflow.OpEquals,
					Value:     "100",
				},
			},
			Actions: []workflow.RuleAction{
				{
					ActionType: workflow.ActionSetProjectStatus,
					Value:      "check",
				},
			},
		},
	}

	configManager := config.NewManager(deps.ConfigPath)

	err = SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
	})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	if project.StatusIcon != "check" {
		t.Errorf("project StatusIcon = %q, want %q", project.StatusIcon, "check")
	}
	if project.StatusIconSource != "auto" {
		t.Errorf("project StatusIconSource = %q, want %q", project.StatusIconSource, "auto")
	}
}

// TestSaveProject_ProjectLevelRulesRespectUserOverride verifies that
// project-level rules do not override a user-set status icon.
func TestSaveProject_ProjectLevelRulesRespectUserOverride(t *testing.T) {
	deps := testutil.SetupTestDeps(t)
	defer deps.Cleanup()

	project := testutil.NewTestProject("ProjOverride")
	project.Path = t.TempDir()
	project.StatusIcon = "star"
	project.StatusIconSource = "user"

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	defer func() { storage.DefaultRepository = oldRepo }()
	deps.Repository.Projects[project.ID] = project

	task1, err := models.NewTask("override-t1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Status = models.StatusDone
	project.Tasks = []*models.Task{task1}
	project.BuildTaskTree()

	cfg := deps.Config
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:      "proj-rule-2",
			Name:    "All Done Override",
			Level:   workflow.RuleLevelProject,
			Enabled: true,
			Triggers: []workflow.RuleTrigger{
				{
					Field:     workflow.FieldCompletionRatio,
					Operation: workflow.OpEquals,
					Value:     "100",
				},
			},
			Actions: []workflow.RuleAction{
				{
					ActionType: workflow.ActionSetProjectStatus,
					Value:      "check",
				},
			},
		},
	}

	configManager := config.NewManager(deps.ConfigPath)

	err = SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
	})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// User override should be preserved
	if project.StatusIcon != "star" {
		t.Errorf("project StatusIcon = %q, want %q (user override)", project.StatusIcon, "star")
	}
	if project.StatusIconSource != "user" {
		t.Errorf("project StatusIconSource = %q, want %q", project.StatusIconSource, "user")
	}
}

// TestSaveProject_WorkflowRuleCompletesRecurringTask verifies that when a
// workflow rule sets a recurring task to "done", GenerateRecurringInstances
// runs after rule evaluation and creates the next occurrence.
func TestSaveProject_WorkflowRuleCompletesRecurringTask(t *testing.T) {
	deps := testutil.SetupTestDeps(t)
	defer deps.Cleanup()

	project := testutil.NewTestProject("Recur")
	project.Path = t.TempDir()

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	defer func() { storage.DefaultRepository = oldRepo }()
	deps.Repository.Projects[project.ID] = project

	// Create a recurring task in "in_progress" status with a recurrence pattern.
	task, err := models.NewTask("recur-task-1", project.GetNextShortID(), models.TaskTypeTask, "Daily standup")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = models.StatusInProgress
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	// Capture previous state (task is in_progress)
	prevStates := workflow.CaptureTaskStates(project.Tasks)

	// Simulate the user completing the task (which triggers the rule evaluation path)
	task.Status = models.StatusDone

	// Create a config with a ConfigManager that doesn't need to persist
	cfg := deps.Config
	configManager := config.NewManager(deps.ConfigPath)

	// Call SaveProject with EvaluateRules enabled and PrevStates
	// (even though there are no matching rules, the post-rule hooks should still run)
	err = SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
		PrevStates:    prevStates,
	})
	if err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// The recurring task should have generated a new instance
	if len(project.Tasks) < 2 {
		t.Fatalf("expected at least 2 tasks (original + recurring instance), got %d", len(project.Tasks))
	}

	// Find the new instance (should be StatusTodo with a recurrence pattern)
	var newInstance *models.Task
	for _, tsk := range project.Tasks {
		if tsk.ID != task.ID && tsk.Status == models.StatusTodo && tsk.Recurrence != nil {
			newInstance = tsk
			break
		}
	}
	if newInstance == nil {
		t.Fatal("recurring instance was not generated after workflow rule evaluation")
	}
	if newInstance.Title != "Daily standup" {
		t.Errorf("new instance title = %q, want %q", newInstance.Title, "Daily standup")
	}

	// The original task should no longer have a recurrence pattern
	if task.Recurrence != nil {
		t.Error("original completed task should have its recurrence cleared")
	}
}
