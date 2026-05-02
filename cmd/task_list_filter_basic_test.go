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
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress),
		testutil.NewTestTaskWithStatus("Task 3", models.StatusDone),
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
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusDone),
		testutil.NewTestTaskWithStatus("Task 3", models.StatusCanceled),
	}

	result := filterTasks(tasks)

	if len(result) != 3 {
		t.Errorf("expected all 3 tasks with --all flag, got %d", len(result))
	}
}
