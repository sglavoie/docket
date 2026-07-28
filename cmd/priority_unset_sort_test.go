package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSortTasks_UnsetPriorityOrdersAsDefault(t *testing.T) {
	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("Unset", models.PriorityUnset),
		testutil.NewTestTaskWithPriority("High", 1),
		testutil.NewTestTaskWithPriority("Medium", models.PriorityDefault),
	}

	err := sortTasks(tasks, "priority")
	testutil.AssertNoError(t, err)

	// Unset resolves to PriorityDefault, so it never precedes P1 and keeps its
	// input order relative to the equally ranked P3 task.
	testutil.AssertEqual(t, tasks[0].Title, "High")
	testutil.AssertEqual(t, tasks[1].Title, "Unset")
	testutil.AssertEqual(t, tasks[2].Title, "Medium")
}

func TestSortFocusTasks_UnsetPriorityOrdersAsDefault(t *testing.T) {
	project := testutil.NewTestProject("Test")
	updated := time.Now()

	mk := func(id string, priority int) focus.FocusTask {
		task := testutil.NewTestTask(id, models.TaskTypeTask)
		task.ID = id
		task.Priority = priority
		task.UpdatedAt = updated
		return focus.FocusTask{Task: task, Project: project}
	}

	tasks := []focus.FocusTask{
		mk("z-unset", models.PriorityUnset),
		mk("a-p3", models.PriorityDefault),
		mk("m-p1", 1),
	}
	sortFocusTasks(tasks)

	for i, want := range []string{"m-p1", "a-p3", "z-unset"} {
		if got := tasks[i].Task.ID; got != want {
			t.Fatalf("focus order[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestNextTasks_UnsetPriorityOrdersAsDefault(t *testing.T) {
	project := testutil.NewTestProject("Test")
	for _, spec := range []struct {
		id       string
		shortID  int
		priority int
	}{
		{"unset", 1, models.PriorityUnset},
		{"p1", 2, 1},
		{"p3", 3, models.PriorityDefault},
	} {
		task, err := models.NewTask(spec.id, spec.shortID, models.TaskTypeTask, spec.id)
		if err != nil {
			t.Fatal(err)
		}
		task.WorkflowState = models.StateNext
		task.Priority = spec.priority
		project.Tasks = append(project.Tasks, task)
	}

	ordered := nextTasks([]*models.Project{project}, nil)

	// Every task is unindexed, so ordering falls to priority then short ID:
	// P1 first, then the unset and P3 tasks tied at PriorityDefault.
	for i, want := range []string{"p1", "unset", "p3"} {
		if got := ordered[i].Task.ID; got != want {
			t.Fatalf("next order[%d] = %q, want %q", i, got, want)
		}
	}
}
