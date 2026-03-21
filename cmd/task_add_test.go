package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskCreationWithParent(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic as potential parent
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic Parent")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	// Create a task that will be a child of the epic
	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)

	// Verify parent assignment
	if task.ParentID == nil {
		t.Error("task should have a parent ID")
	}
	if *task.ParentID != epic.ID {
		t.Errorf("task parent ID = %s, want %s", *task.ParentID, epic.ID)
	}

	// Build tree and verify relationships
	project.BuildTaskTree()
	if task.Parent == nil {
		t.Error("task should have Parent pointer after BuildTaskTree")
	}
	if task.Parent.ID != epic.ID {
		t.Errorf("task.Parent.ID = %s, want %s", task.Parent.ID, epic.ID)
	}
	if len(epic.Children) != 1 {
		t.Errorf("epic should have 1 child, got %d", len(epic.Children))
	}
}

func TestTaskTypeParentValidation(t *testing.T) {
	tests := []struct {
		name       string
		childType  models.TaskType
		parentType models.TaskType
		canBeChild bool
	}{
		{"task under epic", models.TaskTypeTask, models.TaskTypeEpic, true},
		{"story under epic", models.TaskTypeStory, models.TaskTypeEpic, true},
		{"bug under epic", models.TaskTypeBug, models.TaskTypeEpic, true},
		{"idea under epic", models.TaskTypeIdea, models.TaskTypeEpic, true},
		{"task under story", models.TaskTypeTask, models.TaskTypeStory, true},
		{"bug under story", models.TaskTypeBug, models.TaskTypeStory, true},
		{"epic under epic", models.TaskTypeEpic, models.TaskTypeEpic, false},
		{"story under story", models.TaskTypeStory, models.TaskTypeStory, false},
		{"epic under story", models.TaskTypeEpic, models.TaskTypeStory, false},
		{"story under task", models.TaskTypeStory, models.TaskTypeTask, false},
		{"epic under task", models.TaskTypeEpic, models.TaskTypeTask, false},
		{"task under task", models.TaskTypeTask, models.TaskTypeTask, false},
		{"task under bug", models.TaskTypeTask, models.TaskTypeBug, false},
		{"task under idea", models.TaskTypeTask, models.TaskTypeIdea, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.childType.CanBeChildOf(tt.parentType)
			if got != tt.canBeChild {
				t.Errorf("%s.CanBeChildOf(%s) = %v, want %v", tt.childType, tt.parentType, got, tt.canBeChild)
			}
		})
	}
}

func TestTaskPriorityValidation(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)

	// Default priority should be 3
	testutil.AssertEqual(t, task.Priority, 3)

	// Valid priorities
	for p := 1; p <= 5; p++ {
		task.Priority = p
		testutil.AssertEqual(t, task.Priority, p)
	}
}

func TestTaskTagManagement(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)

	// Initially no tags
	testutil.AssertEqual(t, len(task.Tags), 0)

	// Add tags
	task.AddTag("backend")
	task.AddTag("urgent")
	testutil.AssertEqual(t, len(task.Tags), 2)

	// Adding duplicate tag should not duplicate
	task.AddTag("backend")
	testutil.AssertEqual(t, len(task.Tags), 2)

	// Check tag existence
	if !task.HasTag("backend") {
		t.Error("task should have 'backend' tag")
	}
	if !task.HasTag("urgent") {
		t.Error("task should have 'urgent' tag")
	}
	if task.HasTag("frontend") {
		t.Error("task should not have 'frontend' tag")
	}

	// Remove tag
	task.RemoveTag("backend")
	testutil.AssertEqual(t, len(task.Tags), 1)
	if task.HasTag("backend") {
		t.Error("task should not have 'backend' tag after removal")
	}
}

func TestTaskSectionAssignment(t *testing.T) {
	project := testutil.NewTestProject("Test Project")
	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	// Default section should be empty (Active view)
	testutil.AssertEqual(t, task.Section, "")

	// Assign to custom section
	task.Section = "Active"
	testutil.AssertEqual(t, task.Section, "Active")

	// Ensure section exists in project
	project.EnsureSection("Active")
	if !project.HasSection("Active") {
		t.Error("project should have 'Active' section after EnsureSection")
	}
}

func TestTaskShortIDAssignment(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Project starts with NextShortID = 1
	testutil.AssertEqual(t, project.NextShortID, 1)

	// Create first task
	id1 := project.GetNextShortID()
	testutil.AssertEqual(t, id1, 1)
	testutil.AssertEqual(t, project.NextShortID, 2)

	// Create second task
	id2 := project.GetNextShortID()
	testutil.AssertEqual(t, id2, 2)
	testutil.AssertEqual(t, project.NextShortID, 3)
}

func TestNewTaskDefaults(t *testing.T) {
	task, err := models.NewTask("test-id", 42, models.TaskTypeTask, "Test Title")
	if err != nil {
		t.Fatal(err)
	}

	testutil.AssertEqual(t, task.ID, "test-id")
	testutil.AssertEqual(t, task.ShortID, 42)
	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
	testutil.AssertEqual(t, task.Title, "Test Title")
	testutil.AssertEqual(t, task.Status, models.StatusTodo)
	testutil.AssertEqual(t, task.Priority, 3)
	testutil.AssertEqual(t, task.Section, "") // Empty section = Active view
	testutil.AssertEqual(t, task.Description, "")

	if task.ParentID != nil {
		t.Error("new task should have nil ParentID")
	}
	if task.CompletedAt != nil {
		t.Error("new task should have nil CompletedAt")
	}
	if task.DueDate != nil {
		t.Error("new task should have nil DueDate")
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if task.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestTaskTypeHeadingPrefix(t *testing.T) {
	tests := []struct {
		taskType models.TaskType
		prefix   string
	}{
		{models.TaskTypeEpic, "Epic"},
		{models.TaskTypeStory, "Story"},
		{models.TaskTypeTask, "Task"},
		{models.TaskTypeBug, "Bug"},
		{models.TaskTypeIdea, "Idea"},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			got := tt.taskType.HeadingPrefix()
			if got != tt.prefix {
				t.Errorf("TaskType(%q).HeadingPrefix() = %q, want %q", tt.taskType, got, tt.prefix)
			}
		})
	}
}

func TestTaskTypeIcons(t *testing.T) {
	// Each task type should have an icon
	for _, taskType := range models.AllTaskTypes() {
		icon := taskType.Icon()
		if icon == "" || icon == "?" {
			t.Errorf("TaskType(%q) should have a valid icon, got %q", taskType, icon)
		}
	}
}

// resetTaskAddVars resets all package-level flag variables used by runTaskAdd.
func resetTaskAddVars() {
	taskTypeFlag = ""
	taskParentFlag = ""
	taskPriority = 3
	taskTags = nil
	taskSectionFlag = ""
	taskStageFlag = ""
	taskRecurFlag = ""
	taskTemplateFlag = ""
	isEpic = false
	isStory = false
	isBug = false
	isIdea = false
	addTypeAttrs = TypeAttrValues{}
	taskCreateSectionFlag = false
}

// setupTaskAddTest creates common test dependencies and returns them.
// Caller must defer deps.Cleanup() and restore storage/viper.
func setupTaskAddTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetTaskAddVars()

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

func TestRunTaskAdd(t *testing.T) {
	t.Run("creates task successfully", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)

		err := runTaskAdd(taskAddCmd, []string{"My New Task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Title, "My New Task")
		testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
		testutil.AssertEqual(t, task.Priority, 3)
		testutil.AssertEqual(t, task.Status, models.StatusTodo)
	})

	t.Run("rejects missing title", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		err := runTaskAdd(taskAddCmd, []string{})
		if err == nil {
			t.Error("expected error for missing title")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		err := runTaskAdd(taskAddCmd, []string{" "})
		if err == nil {
			t.Error("expected error for empty title")
		}
	})

	t.Run("creates epic with flag", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		isEpic = true

		err := runTaskAdd(taskAddCmd, []string{"My Epic"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		testutil.AssertEqual(t, savedProject.Tasks[len(savedProject.Tasks)-1].Type, models.TaskTypeEpic)
	})

	t.Run("creates bug with flag", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		isBug = true

		err := runTaskAdd(taskAddCmd, []string{"Login crash"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		testutil.AssertEqual(t, savedProject.Tasks[len(savedProject.Tasks)-1].Type, models.TaskTypeBug)
	})

	t.Run("sets priority", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskPriority = 1

		err := runTaskAdd(taskAddCmd, []string{"Urgent task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		testutil.AssertEqual(t, savedProject.Tasks[len(savedProject.Tasks)-1].Priority, 1)
	})

	t.Run("sets tags", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskTags = []string{"backend", "api"}

		err := runTaskAdd(taskAddCmd, []string{"Tagged task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		if !task.HasTag("backend") {
			t.Error("task should have 'backend' tag")
		}
		if !task.HasTag("api") {
			t.Error("task should have 'api' tag")
		}
	})

	t.Run("creates task with section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "Backlog"
		taskCreateSectionFlag = true
		// Simulate the flag being explicitly set via command line
		_ = taskAddCmd.Flags().Set("section", "Backlog")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
			taskCreateSectionFlag = false
		}()

		err := runTaskAdd(taskAddCmd, []string{"Backlog task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		testutil.AssertEqual(t, savedProject.Tasks[len(savedProject.Tasks)-1].Section, "Backlog")
		if !savedProject.HasSection("Backlog") {
			t.Error("project should have 'Backlog' section after task creation")
		}
	})

	t.Run("rejects invalid recurrence", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskRecurFlag = "invalid"

		err := runTaskAdd(taskAddCmd, []string{"Recurring task"})
		if err == nil {
			t.Error("expected error for invalid recurrence pattern")
		}
	})

	t.Run("creates task with valid recurrence", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskRecurFlag = "weekly"

		err := runTaskAdd(taskAddCmd, []string{"Weekly task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		if task.Recurrence == nil {
			t.Fatal("task should have recurrence set")
		}
	})
}

func TestRunTaskAdd_SectionValidation(t *testing.T) {
	t.Run("rejects nonexistent section without --create-section", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskSectionFlag = "Nonexistent"
		_ = taskAddCmd.Flags().Set("section", "Nonexistent")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		if err == nil {
			t.Error("expected error for nonexistent section without --create-section")
		}
	})

	t.Run("accepts nonexistent section with --create-section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "NewSection"
		taskCreateSectionFlag = true
		_ = taskAddCmd.Flags().Set("section", "NewSection")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
			taskCreateSectionFlag = false
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		if !savedProject.HasSection("NewSection") {
			t.Error("expected NewSection to be created")
		}
	})

	t.Run("accepts existing section without --create-section", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		// "Next" is the default section
		initialCount := len(project.Tasks)
		taskSectionFlag = "Next"
		_ = taskAddCmd.Flags().Set("section", "Next")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"My task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
	})
}
