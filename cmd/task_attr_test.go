package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetTaskAttrVars() {
	attrTypeAttrs = TypeAttrValues{}
	attrClear = false
}

func setupTaskAttrTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetTaskAttrVars()

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

func TestApplyTypeAttrValues(t *testing.T) {
	t.Run("sets bug attributes on bug task", func(t *testing.T) {
		task := testutil.NewTestTask("Fix crash", models.TaskTypeBug)
		vals := TypeAttrValues{
			BugSeverity:     "major",
			BugReproducible: "yes",
			BugWorkaround:   "use x",
		}
		err := applyTypeAttrValues(task, vals)
		testutil.AssertNoError(t, err)
		if task.BugAttrs == nil {
			t.Fatal("expected BugAttrs to be set")
		}
		testutil.AssertEqual(t, string(task.BugAttrs.Severity), "major")
		testutil.AssertEqual(t, string(task.BugAttrs.Reproducible), "yes")
		testutil.AssertEqual(t, task.BugAttrs.Workaround, "use x")
	})

	t.Run("rejects bug.* on non-bug task", func(t *testing.T) {
		task := testutil.NewTestTask("Some task", models.TaskTypeTask)
		vals := TypeAttrValues{BugSeverity: "major"}
		err := applyTypeAttrValues(task, vals)
		if err == nil {
			t.Error("expected error when applying bug attrs to non-bug task")
		}
	})

	t.Run("sets idea attributes on idea task", func(t *testing.T) {
		task := testutil.NewTestTask("Some idea", models.TaskTypeIdea)
		vals := TypeAttrValues{
			IdeaCategory: "feature",
			IdeaValue:    "high",
			IdeaEffort:   "small",
		}
		err := applyTypeAttrValues(task, vals)
		testutil.AssertNoError(t, err)
		if task.IdeaAttrs == nil {
			t.Fatal("expected IdeaAttrs to be set")
		}
		testutil.AssertEqual(t, string(task.IdeaAttrs.Category), "feature")
		testutil.AssertEqual(t, string(task.IdeaAttrs.Value), "high")
		testutil.AssertEqual(t, string(task.IdeaAttrs.Effort), "small")
	})

	t.Run("rejects idea.* on non-idea task", func(t *testing.T) {
		task := testutil.NewTestTask("Some task", models.TaskTypeTask)
		vals := TypeAttrValues{IdeaCategory: "feature"}
		err := applyTypeAttrValues(task, vals)
		if err == nil {
			t.Error("expected error when applying idea attrs to non-idea task")
		}
	})

	t.Run("sets task attributes on task type", func(t *testing.T) {
		task := testutil.NewTestTask("Some task", models.TaskTypeTask)
		vals := TypeAttrValues{
			TaskEffort:   "small",
			TaskWorkType: "development",
		}
		err := applyTypeAttrValues(task, vals)
		testutil.AssertNoError(t, err)
		if task.TaskAttrs == nil {
			t.Fatal("expected TaskAttrs to be set")
		}
		testutil.AssertEqual(t, string(task.TaskAttrs.Effort), "small")
		testutil.AssertEqual(t, string(task.TaskAttrs.WorkType), "development")
	})

	t.Run("rejects task.* on non-task type", func(t *testing.T) {
		task := testutil.NewTestTask("Some bug", models.TaskTypeBug)
		vals := TypeAttrValues{TaskEffort: "small"}
		err := applyTypeAttrValues(task, vals)
		if err == nil {
			t.Error("expected error when applying task attrs to non-task task")
		}
	})

	t.Run("sets story attributes on story type", func(t *testing.T) {
		task := testutil.NewTestTask("Some story", models.TaskTypeStory)
		vals := TypeAttrValues{
			StoryPointsSet: true,
			StoryPoints:    5,
		}
		err := applyTypeAttrValues(task, vals)
		testutil.AssertNoError(t, err)
		if task.StoryAttrs == nil {
			t.Fatal("expected StoryAttrs to be set")
		}
		testutil.AssertEqual(t, task.StoryAttrs.StoryPoints, 5)
	})

	t.Run("rejects story.* on non-story type", func(t *testing.T) {
		task := testutil.NewTestTask("Some task", models.TaskTypeTask)
		vals := TypeAttrValues{StoryPointsSet: true, StoryPoints: 5}
		err := applyTypeAttrValues(task, vals)
		if err == nil {
			t.Error("expected error when applying story attrs to non-story task")
		}
	})

	t.Run("sets epic attributes on epic type", func(t *testing.T) {
		task := testutil.NewTestTask("Some epic", models.TaskTypeEpic)
		vals := TypeAttrValues{
			EpicScope:     "defined",
			EpicMilestone: "Q1",
		}
		err := applyTypeAttrValues(task, vals)
		testutil.AssertNoError(t, err)
		if task.EpicAttrs == nil {
			t.Fatal("expected EpicAttrs to be set")
		}
		testutil.AssertEqual(t, string(task.EpicAttrs.Scope), "defined")
		testutil.AssertEqual(t, task.EpicAttrs.Milestone, "Q1")
	})

	t.Run("rejects epic.* on non-epic type", func(t *testing.T) {
		task := testutil.NewTestTask("Some task", models.TaskTypeTask)
		vals := TypeAttrValues{EpicScope: "defined"}
		err := applyTypeAttrValues(task, vals)
		if err == nil {
			t.Error("expected error when applying epic attrs to non-epic task")
		}
	})
}

func TestRunTaskAttr(t *testing.T) {
	t.Run("rejects when no flags set", func(t *testing.T) {
		deps, _ := setupTaskAttrTest(t)
		defer deps.Cleanup()

		// No flags set — anyFlagSet will be false
		err := runTaskAttr(taskAttrCmd, []string{"6"})
		if err == nil {
			t.Error("expected error when no attribute flags are provided")
		}
	})

	t.Run("sets bug severity on a bug task", func(t *testing.T) {
		deps, project := setupTaskAttrTest(t)
		defer deps.Cleanup()

		// ShortID 6 is the Bug "Fix login redirect"
		attrTypeAttrs = TypeAttrValues{BugSeverity: "major"}

		err := runTaskAttr(taskAttrCmd, []string{"6"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		var bugTask *models.Task
		for _, task := range savedProject.Tasks {
			if task.ShortID == 6 {
				bugTask = task
				break
			}
		}
		if bugTask == nil {
			t.Fatal("bug task with ShortID 6 not found")
		}
		if bugTask.BugAttrs == nil {
			t.Fatal("expected BugAttrs to be set after update")
		}
		testutil.AssertEqual(t, string(bugTask.BugAttrs.Severity), "major")
	})

	t.Run("clears attributes with attrClear flag", func(t *testing.T) {
		deps, project := setupTaskAttrTest(t)
		defer deps.Cleanup()

		// First set an attribute on the bug task
		bugTask := project.Tasks[5] // ShortID 6
		bugTask.EnsureBugAttrs().Severity = models.BugSeverity("major")
		deps.Repository.Projects[project.ID] = project

		// Now clear
		attrClear = true

		err := runTaskAttr(taskAttrCmd, []string{"6"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		var saved *models.Task
		for _, task := range savedProject.Tasks {
			if task.ShortID == 6 {
				saved = task
				break
			}
		}
		if saved == nil {
			t.Fatal("bug task with ShortID 6 not found after clear")
		}
		if saved.BugAttrs != nil {
			t.Error("expected BugAttrs to be nil after clear")
		}
	})

	t.Run("rejects nonexistent task", func(t *testing.T) {
		deps, _ := setupTaskAttrTest(t)
		defer deps.Cleanup()

		attrTypeAttrs = TypeAttrValues{BugSeverity: "major"}

		err := runTaskAttr(taskAttrCmd, []string{"9999"})
		if err == nil {
			t.Error("expected error for nonexistent task ID")
		}
	})
}
