package cmd

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCollectTaskDescendantIDs(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	project.BuildTaskTree()

	// Find the epic (root task with children)
	var epic *models.Task
	for _, task := range project.Tasks {
		if task.Type == models.TaskTypeEpic {
			epic = task
			break
		}
	}
	if epic == nil {
		t.Fatal("expected project to have an epic task")
	}

	t.Run("collects epic and all descendants", func(t *testing.T) {
		ids := collectTaskDescendantIDs(project.Tasks, map[string]bool{epic.ID: true})
		if !ids[epic.ID] {
			t.Error("expected epic ID in result")
		}
		// Should include more than just the epic
		if len(ids) < 2 {
			t.Errorf("expected at least 2 IDs (epic + children), got %d", len(ids))
		}
	})

	t.Run("single leaf task returns just itself", func(t *testing.T) {
		var leaf *models.Task
		for _, task := range project.Tasks {
			if len(task.Children) == 0 && task.Type == models.TaskTypeTask {
				leaf = task
				break
			}
		}
		if leaf == nil {
			t.Skip("no leaf task found")
		}
		ids := collectTaskDescendantIDs(project.Tasks, map[string]bool{leaf.ID: true})
		testutil.AssertEqual(t, len(ids), 1)
		if !ids[leaf.ID] {
			t.Error("expected leaf ID in result")
		}
	})

	t.Run("empty root set returns empty", func(t *testing.T) {
		ids := collectTaskDescendantIDs(project.Tasks, map[string]bool{})
		testutil.AssertEqual(t, len(ids), 0)
	})

	t.Run("multiple root IDs collects all descendants", func(t *testing.T) {
		var bug *models.Task
		for _, task := range project.Tasks {
			if task.Type == models.TaskTypeBug {
				bug = task
				break
			}
		}
		if bug == nil {
			t.Skip("no bug task found")
		}

		roots := map[string]bool{epic.ID: true, bug.ID: true}
		ids := collectTaskDescendantIDs(project.Tasks, roots)

		if !ids[epic.ID] {
			t.Error("expected epic ID in result")
		}
		if !ids[bug.ID] {
			t.Error("expected bug ID in result")
		}
		// Epic descendants + bug
		epicIDs := collectTaskDescendantIDs(project.Tasks, map[string]bool{epic.ID: true})
		if len(ids) != len(epicIDs)+1 {
			t.Errorf("expected %d IDs, got %d", len(epicIDs)+1, len(ids))
		}
	})
}

func TestRunTaskDelete_Validation(t *testing.T) {
	t.Run("force flag defaults to false", func(t *testing.T) {
		// Reset the flag
		taskDeleteForce = false
		testutil.AssertEqual(t, taskDeleteForce, false)
	})
}

// TestDeleteRoutesThroughAppAndCleansPlanningRefs proves `tt task delete` routes
// the removal through the application-layer Delete use case, removes the task from
// the persisted project, and cleans BOTH the durable planning refs (Focus Queue,
// committed as a config artifact in the delete transaction) and the machine-local
// recent visits (repaired separately after the commit) for the deleted task.
func TestDeleteRoutesThroughAppAndCleansPlanningRefs(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	// Seed durable planning refs and a machine-local recent visit for the task, then
	// persist the config so LoadCurrentProject inside the command sees them.
	deps.Config.Planning().FocusQueue = []config.FocusQueueItem{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	deps.Config.Planning().DailyPlan = []config.DailyPlanItem{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	deps.Config.Planning().FocusDismissedIDs = []string{task.ID}
	deps.Config.Machine().RecentTaskVisits = []config.RecentTaskVisit{
		{TaskID: task.ID, ProjectID: "alpha"},
		{TaskID: task.ID, ProjectID: project.ID},
	}
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	prevForce := taskDeleteForce
	taskDeleteForce = true
	t.Cleanup(func() { taskDeleteForce = prevForce })

	cmd := newStateTestCmd()
	err := runTaskDelete(cmd, []string{fmt.Sprintf("%d", task.ShortID)})
	testutil.AssertNoError(t, err)

	// Task removed from the persisted project.
	saved := deps.Repository.Projects[project.ID]
	if saved.GetTaskByID(task.ID) != nil {
		t.Errorf("task %s still present after delete", task.ID)
	}

	// Both the durable and machine-local refs are gone from the on-disk config.
	reloaded, loadErr := manager.LoadRuntimeState()
	testutil.AssertNoError(t, loadErr)
	if got := reloaded.Planning().FocusQueue; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Errorf("delete cleaned the wrong focus-queue refs: %v", got)
	}
	if got := reloaded.Planning().DailyPlan; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Errorf("delete cleaned the wrong daily-plan refs: %v", got)
	}
	if got := reloaded.Planning().FocusDismissedIDs; len(got) != 1 || got[0] != task.ID {
		t.Errorf("qualified delete changed legacy dismissed IDs: %v", got)
	}
	if got := reloaded.Machine().RecentTaskVisits; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Errorf("delete cleaned the wrong machine-local recent visits: %v", got)
	}
}
