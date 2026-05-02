package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_ByPriority(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 2

	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("Priority 1 task", 1),
		testutil.NewTestTaskWithPriority("Priority 2 task", 2),
		testutil.NewTestTaskWithPriority("Priority 3 task", 3),
		testutil.NewTestTaskWithPriority("Another priority 2 task", 2),
	}

	result := filterTasks(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 tasks with priority 2, got %d", len(result))
	}
	for _, task := range result {
		if task.Priority != 2 {
			t.Errorf("expected priority 2, got %d for task %s", task.Priority, task.Title)
		}
	}
}

func TestFilterTasks_ZeroPriorityIgnored(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listPriority = 0

	tasks := []*models.Task{
		testutil.NewTestTaskWithPriority("P1 task", 1),
		testutil.NewTestTaskWithPriority("P2 task", 2),
		testutil.NewTestTaskWithPriority("P0 task", 0),
	}

	result := filterTasks(tasks)

	if len(result) != 3 {
		t.Errorf("expected all 3 tasks when priority=0, got %d", len(result))
	}
}
