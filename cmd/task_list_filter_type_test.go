package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_ByType(t *testing.T) {
	tests := []struct {
		name         string
		taskType     string
		expectedType models.TaskType
	}{
		{"epic", "epic", models.TaskTypeEpic},
		{"story", "story", models.TaskTypeStory},
		{"task", "task", models.TaskTypeTask},
		{"bug", "bug", models.TaskTypeBug},
		{"idea", "idea", models.TaskTypeIdea},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer resetFilterVars()
			listShowAll = true
			listTaskType = tc.taskType

			tasks := []*models.Task{
				testutil.NewTestTask("Epic task", models.TaskTypeEpic),
				testutil.NewTestTask("Story task", models.TaskTypeStory),
				testutil.NewTestTask("Regular task", models.TaskTypeTask),
				testutil.NewTestTask("Bug task", models.TaskTypeBug),
				testutil.NewTestTask("Idea task", models.TaskTypeIdea),
			}

			result := filterTasks(tasks)

			if len(result) != 1 {
				t.Errorf("expected 1 task of type %s, got %d", tc.taskType, len(result))
			}
			if len(result) > 0 && result[0].Type != tc.expectedType {
				t.Errorf("expected task type %s, got %s", tc.expectedType, result[0].Type)
			}
		})
	}
}

func TestFilterTasks_InvalidType(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "invalid_type"

	tasks := []*models.Task{
		testutil.NewTestTask("Task 1", models.TaskTypeTask),
		testutil.NewTestTask("Task 2", models.TaskTypeTask),
	}

	result := filterTasks(tasks)

	if len(result) != 0 {
		t.Errorf("expected 0 tasks with invalid type, got %d", len(result))
	}
}

func TestTaskMatchesFilters_ByType(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskType = "bug"

	bug := testutil.NewTestTask("A bug", models.TaskTypeBug)
	task := testutil.NewTestTask("A task", models.TaskTypeTask)

	if !taskMatchesFilters(bug) {
		t.Error("expected bug to match type=bug filter")
	}
	if taskMatchesFilters(task) {
		t.Error("expected task to NOT match type=bug filter")
	}
}
