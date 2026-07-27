package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSortTasks_ByPriority(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("Low", 3),
		testutil.NewTestTaskWithPriority("High", 1),
		testutil.NewTestTaskWithPriority("Medium", 2),
	}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "High")
	testutil.AssertEqual(t, tasks[1].Title, "Medium")
	testutil.AssertEqual(t, tasks[2].Title, "Low")
}

func TestSortTasks_ByCreated(t *testing.T) {
	now := time.Now()

	t1 := testutil.NewTestTask("Oldest", models.TaskTypeTask)
	t1.CreatedAt = now.Add(-2 * time.Hour)

	t2 := testutil.NewTestTask("Newest", models.TaskTypeTask)
	t2.CreatedAt = now

	t3 := testutil.NewTestTask("Middle", models.TaskTypeTask)
	t3.CreatedAt = now.Add(-1 * time.Hour)

	tasks := []*models.Task{t1, t2, t3}

	err := sortTasks(tasks, "created")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "Newest")
	testutil.AssertEqual(t, tasks[1].Title, "Middle")
	testutil.AssertEqual(t, tasks[2].Title, "Oldest")
}

func TestSortTasks_ByUpdated(t *testing.T) {
	now := time.Now()

	t1 := testutil.NewTestTask("Oldest", models.TaskTypeTask)
	t1.UpdatedAt = now.Add(-2 * time.Hour)

	t2 := testutil.NewTestTask("Newest", models.TaskTypeTask)
	t2.UpdatedAt = now

	t3 := testutil.NewTestTask("Middle", models.TaskTypeTask)
	t3.UpdatedAt = now.Add(-1 * time.Hour)

	tasks := []*models.Task{t1, t2, t3}

	err := sortTasks(tasks, "updated")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "Newest")
	testutil.AssertEqual(t, tasks[1].Title, "Middle")
	testutil.AssertEqual(t, tasks[2].Title, "Oldest")
}

func TestSortTasks_ByCanonicalStateOrder(t *testing.T) {
	states := models.AllWorkflowStates()
	tasks := make([]*models.Task, 0, len(states))
	for i := len(states) - 1; i >= 0; i-- {
		task := testutil.NewTestTask(states[i].String(), models.TaskTypeTask)
		task.ApplyWorkflowState(states[i])
		tasks = append(tasks, task)
	}

	err := sortTasks(tasks, "state")
	testutil.AssertNoError(t, err)

	for i, want := range states {
		if got := tasks[i].State(); got != want {
			t.Errorf("state at %d = %q, want %q", i, got, want)
		}
	}
}

func TestSortTasks_ByTitle(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTask("Charlie", models.TaskTypeTask),
		testutil.NewTestTask("alpha", models.TaskTypeTask),
		testutil.NewTestTask("Bravo", models.TaskTypeTask),
	}

	err := sortTasks(tasks, "title")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "alpha")
	testutil.AssertEqual(t, tasks[1].Title, "Bravo")
	testutil.AssertEqual(t, tasks[2].Title, "Charlie")
}

func TestSortTasks_InvalidField(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTask("Task 1", models.TaskTypeTask),
	}

	err := sortTasks(tasks, "nonexistent")
	if err == nil {
		t.Error("expected error for invalid sort field, got nil")
	}
}

func TestSortTasks_EmptySlice(t *testing.T) {
	tasks := []*models.Task{}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, len(tasks), 0)
}

func TestSortTasks_StableSort(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("First", 1),
		testutil.NewTestTaskWithPriority("Second", 1),
		testutil.NewTestTaskWithPriority("Third", 1),
	}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, tasks[0].Title, "First")
	testutil.AssertEqual(t, tasks[1].Title, "Second")
	testutil.AssertEqual(t, tasks[2].Title, "Third")
}

func TestStateOrderCoversAllStates(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		if _, ok := canonicalStateOrder[state]; !ok {
			t.Errorf("canonicalStateOrder map is missing state %q", state)
		}
	}
}
