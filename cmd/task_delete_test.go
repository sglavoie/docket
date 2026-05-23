package cmd

import (
	"testing"

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

func TestFilterDeletedTasks_UnblocksTaskWhenLastBlockerDeleted(t *testing.T) {
	blocker, err := models.NewTask("blocker-id", 1, models.TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := models.NewTask("dep-id", 2, models.TaskTypeTask, "Dependent")
	if err != nil {
		t.Fatal(err)
	}
	if depErr := dependent.AddDependency(blocker.ID); depErr != nil {
		t.Fatal(depErr)
	}
	dependent.Status = models.StatusBlocked

	idsToDelete := map[string]bool{blocker.ID: true}
	remaining := filterDeletedTasks([]*models.Task{blocker, dependent}, idsToDelete)

	if len(remaining) != 1 {
		t.Fatalf("expected 1 surviving task, got %d", len(remaining))
	}
	if len(remaining[0].DependsOn) != 0 {
		t.Errorf("expected DependsOn to be cleared, got %v", remaining[0].DependsOn)
	}
	if remaining[0].Status == models.StatusBlocked {
		t.Errorf("expected dependent to be unblocked after last blocker deleted, still %s", remaining[0].Status)
	}
}

func TestFilterDeletedTasks_KeepsBlockedWhenOtherBlockersRemain(t *testing.T) {
	b1, _ := models.NewTask("b1", 1, models.TaskTypeTask, "B1")
	b2, _ := models.NewTask("b2", 2, models.TaskTypeTask, "B2")
	dependent, _ := models.NewTask("dep", 3, models.TaskTypeTask, "Dep")
	_ = dependent.AddDependency(b1.ID)
	_ = dependent.AddDependency(b2.ID)
	dependent.Status = models.StatusBlocked

	idsToDelete := map[string]bool{b1.ID: true}
	remaining := filterDeletedTasks([]*models.Task{b1, b2, dependent}, idsToDelete)

	var dep *models.Task
	for _, t := range remaining {
		if t.ID == dependent.ID {
			dep = t
		}
	}
	if dep == nil {
		t.Fatal("dependent task missing from remaining")
	}
	if dep.Status != models.StatusBlocked {
		t.Errorf("expected dependent to remain blocked (other blocker still present), got %s", dep.Status)
	}
	if len(dep.DependsOn) != 1 || dep.DependsOn[0] != b2.ID {
		t.Errorf("expected DependsOn to contain only b2, got %v", dep.DependsOn)
	}
}
