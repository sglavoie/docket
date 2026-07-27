package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_EmptyTaskList(t *testing.T) {
	defer resetFilterVars()

	tasks := []*models.Task{}
	result := filterTasks(tasks)

	if len(result) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result))
	}
}

func TestFilterTasks_NoFilters(t *testing.T) {
	defer resetFilterVars()

	tasks := []*models.Task{
		testutil.NewTestTaskWithState("Task 1", models.StateReady),
		testutil.NewTestTaskWithState("Task 2", models.StateInProgress),
		testutil.NewTestTaskWithState("Task 3", models.StateDone),
	}

	result := filterTasks(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 active tasks, got %d", len(result))
	}
}

func TestFilterTasks_ShowAll(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true

	tasks := []*models.Task{
		testutil.NewTestTaskWithState("Task 1", models.StateReady),
		testutil.NewTestTaskWithState("Task 2", models.StateDone),
		testutil.NewTestTaskWithState("Task 3", models.StateCanceled),
	}

	result := filterTasks(tasks)

	if len(result) != 3 {
		t.Errorf("expected all 3 tasks with --all flag, got %d", len(result))
	}
}
