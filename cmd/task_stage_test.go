package cmd

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskStage(t *testing.T) {
	t.Run("single arg shows stage without modifying task", func(t *testing.T) {
		deps, project := setupProjectTest(t)

		// Set stage on the first existing task
		task := project.Tasks[0]
		task.Stage = "Design"
		deps.Repository.Projects[project.ID] = project

		// Reset the repo's save counter so we can detect spurious saves
		deps.Repository.LastSaved = nil

		// Run with only 1 arg (task ID) -- should show stage, not clear it
		err := runTaskStage(taskStageCmd, []string{fmt.Sprintf("%d", task.ShortID)})
		testutil.AssertNoError(t, err)

		// Verify the stage was NOT modified
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Stage != "Design" {
			t.Errorf("stage should still be 'Design', got %q", savedTask.Stage)
		}

		// Verify no save was triggered
		if deps.Repository.LastSaved != nil {
			t.Error("single-arg stage command should not trigger a save")
		}
	})

	t.Run("single arg with no stage shows no-stage message", func(t *testing.T) {
		_, project := setupProjectTest(t)

		// Use the first existing task with no stage
		task := project.Tasks[0]
		task.Stage = ""

		err := runTaskStage(taskStageCmd, []string{fmt.Sprintf("%d", task.ShortID)})
		testutil.AssertNoError(t, err)
	})

	t.Run("two args sets stage on task", func(t *testing.T) {
		deps, project := setupProjectTest(t)

		task := project.Tasks[0]
		task.Stage = ""
		deps.Repository.Projects[project.ID] = project

		err := runTaskStage(taskStageCmd, []string{fmt.Sprintf("%d", task.ShortID), "Implement"})
		testutil.AssertNoError(t, err)

		// Verify the stage was set
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask == nil {
			t.Fatal("task not found after save")
		}
		if savedTask.Stage != "Implement" {
			t.Errorf("stage should be 'Implement', got %q", savedTask.Stage)
		}
	})

	t.Run("two args with empty string clears stage", func(t *testing.T) {
		deps, project := setupProjectTest(t)

		task := project.Tasks[0]
		task.Stage = "Design"
		deps.Repository.Projects[project.ID] = project

		err := runTaskStage(taskStageCmd, []string{fmt.Sprintf("%d", task.ShortID), ""})
		testutil.AssertNoError(t, err)

		// Verify the stage was cleared
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Stage != "" {
			t.Errorf("stage should be empty after clearing, got %q", savedTask.Stage)
		}
	})
}

func TestTaskStage_ReadOnlyDoesNotClear(t *testing.T) {
	// Regression test: before the fix, running 'tt task stage <id>' with no
	// stage argument would silently clear the stage instead of showing it.
	deps, project := setupProjectTest(t)

	task := project.Tasks[0]
	task.Stage = "Review"
	deps.Repository.Projects[project.ID] = project

	err := runTaskStage(taskStageCmd, []string{fmt.Sprintf("%d", task.ShortID)})
	testutil.AssertNoError(t, err)

	// The stage must still be "Review" -- the single-arg invocation is read-only
	savedProject := deps.Repository.Projects[project.ID]
	savedTask := savedProject.GetTaskByShortID(task.ShortID)
	if savedTask.Stage != "Review" {
		t.Errorf("stage should still be 'Review' after read-only invocation, got %q", savedTask.Stage)
	}
}
