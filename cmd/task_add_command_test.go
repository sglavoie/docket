package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

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

	t.Run("creates task with custom section in CustomSection field", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		initialCount := len(project.Tasks)
		taskSectionFlag = "Backlog"
		taskCreateSectionFlag = true
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
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.CustomSection, "Backlog")
		testutil.AssertEqual(t, task.Section, "")
		if !savedProject.HasSection("Backlog") {
			t.Error("project should have 'Backlog' section after task creation")
		}
	})

	t.Run("creates task with GTD section in Section field", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		project.EnsureDefaultSections()

		initialCount := len(project.Tasks)
		taskSectionFlag = "Inbox"
		_ = taskAddCmd.Flags().Set("section", "Inbox")
		defer func() {
			taskAddCmd.Flags().Lookup("section").Changed = false
			taskSectionFlag = ""
		}()

		err := runTaskAdd(taskAddCmd, []string{"Inbox task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Section, "Inbox")
		testutil.AssertEqual(t, task.CustomSection, "")
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

	t.Run("whitespace-only stage is not persisted", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskStageFlag = "   "
		defer func() { taskStageFlag = "" }()

		err := runTaskAdd(taskAddCmd, []string{"Stage whitespace task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Stage, "")
	})

	t.Run("stage value is trimmed before persisting", func(t *testing.T) {
		deps, project := setupTaskAddTest(t)
		defer deps.Cleanup()

		taskStageFlag = "  Design\t"
		defer func() { taskStageFlag = "" }()

		err := runTaskAdd(taskAddCmd, []string{"Padded stage task"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Stage, "Design")
	})

	t.Run("template flag is trimmed before lookup", func(t *testing.T) {
		deps, _ := setupTaskAddTest(t)
		defer deps.Cleanup()

		// No template is registered, so a padded missing ID must surface the
		// trimmed identifier in the error rather than the raw whitespace form.
		taskTemplateFlag = "  unknown-template  "
		defer func() { taskTemplateFlag = "" }()

		err := runTaskAdd(taskAddCmd, []string{"Templated task"})
		if err == nil {
			t.Fatal("expected error for unknown template")
		}
		if !strings.Contains(err.Error(), "unknown-template") || strings.Contains(err.Error(), "  unknown-template") {
			t.Errorf("expected trimmed template id in error, got %q", err.Error())
		}
	})
}
