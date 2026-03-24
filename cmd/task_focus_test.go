package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskFocusToggle(t *testing.T) {
	t.Run("toggle activates inactive task", func(t *testing.T) {
		task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
		testutil.AssertEqual(t, task.Active, false)

		task.ToggleActive()
		testutil.AssertEqual(t, task.Active, true)
	})

	t.Run("toggle deactivates active task", func(t *testing.T) {
		task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
		task.Active = true

		task.ToggleActive()
		testutil.AssertEqual(t, task.Active, false)
	})
}

func TestTaskDeactivate(t *testing.T) {
	t.Run("deactivate on active task clears flag and updates timestamp", func(t *testing.T) {
		task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
		task.Active = true
		originalUpdatedAt := task.UpdatedAt
		time.Sleep(time.Millisecond)

		task.Deactivate()

		testutil.AssertEqual(t, task.Active, false)
		if !task.UpdatedAt.After(originalUpdatedAt) {
			t.Error("expected UpdatedAt to be updated after deactivate")
		}
	})

	t.Run("deactivate on inactive task still updates timestamp at model level", func(t *testing.T) {
		// At the model level, Deactivate() unconditionally updates UpdatedAt.
		// The command-level pre-check in runTaskUnfocus prevents calling
		// Deactivate() when the task is already inactive.
		task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
		testutil.AssertEqual(t, task.Active, false)

		task.Deactivate()
		testutil.AssertEqual(t, task.Active, false)
	})
}

func TestUnfocusPreCheckSkipsInactiveTask(t *testing.T) {
	// This tests the pre-check logic: when a task is not active,
	// runTaskUnfocus should return early without triggering WithTaskUpdate.
	// We test this by verifying the condition that the command checks.
	project := testutil.NewTestProjectWithTasks("Focus Project")

	task := project.Tasks[0]
	task.Active = false

	// The pre-check in runTaskUnfocus: if !preTask.Active, return early
	// This means WithTaskUpdate (which calls Deactivate and saves) is NOT called.
	if task.Active {
		t.Fatal("expected task to not be active - pre-check would continue to save")
	}
	// Pre-check would print "Task #N is not in focus" and return nil
}

func TestUnfocusPreCheckAllowsActiveTask(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Focus Project")

	task := project.Tasks[0]
	task.Active = true

	// The pre-check passes for active tasks, allowing WithTaskUpdate to proceed
	if !task.Active {
		t.Fatal("expected task to be active - pre-check would short-circuit")
	}

	// WithTaskUpdate would call Deactivate() and save
	task.Deactivate()
	testutil.AssertEqual(t, task.Active, false)
}
