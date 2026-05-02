package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_ByTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTag = "backend"

	task1 := testutil.NewTestTask("Task with backend tag", models.TaskTypeTask)
	task1.Tags = []string{"backend", "api"}

	task2 := testutil.NewTestTask("Task with frontend tag", models.TaskTypeTask)
	task2.Tags = []string{"frontend", "ui"}

	task3 := testutil.NewTestTask("Task with both tags", models.TaskTypeTask)
	task3.Tags = []string{"backend", "frontend"}

	tasks := []*models.Task{task1, task2, task3}

	result := filterTasks(tasks)

	if len(result) != 2 {
		t.Errorf("expected 2 tasks with 'backend' tag, got %d", len(result))
	}
}

func TestTaskMatchesFilters_ByTag(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTag = "urgent"

	tagged := testutil.NewTestTask("Tagged", models.TaskTypeTask)
	tagged.Tags = []string{"urgent"}
	untagged := testutil.NewTestTask("Untagged", models.TaskTypeTask)
	untagged.Tags = []string{"low"}

	if !taskMatchesFilters(tagged) {
		t.Error("expected tagged task to match")
	}
	if taskMatchesFilters(untagged) {
		t.Error("expected untagged task to NOT match")
	}
}
