package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestFilterTasks_ByStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		expectedStatus models.TaskStatus
	}{
		{"todo", "todo", models.StatusTodo},
		{"in_progress", "in_progress", models.StatusInProgress},
		{"review", "review", models.StatusReview},
		{"standby", "standby", models.StatusStandby},
		{"maybe", "maybe", models.StatusMaybe},
		{"done", "done", models.StatusDone},
		{"blocked", "blocked", models.StatusBlocked},
		{"canceled", "canceled", models.StatusCanceled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer resetFilterVars()
			listShowAll = true
			listTaskStatus = tc.status

			tasks := []*models.Task{
				testutil.NewTestTaskWithStatus("Todo task", models.StatusTodo),
				testutil.NewTestTaskWithStatus("In Progress task", models.StatusInProgress),
				testutil.NewTestTaskWithStatus("Review task", models.StatusReview),
				testutil.NewTestTaskWithStatus("Standby task", models.StatusStandby),
				testutil.NewTestTaskWithStatus("Maybe task", models.StatusMaybe),
				testutil.NewTestTaskWithStatus("Done task", models.StatusDone),
				testutil.NewTestTaskWithStatus("Blocked task", models.StatusBlocked),
				testutil.NewTestTaskWithStatus("Canceled task", models.StatusCanceled),
			}

			result := filterTasks(tasks)

			if len(result) != 1 {
				t.Errorf("expected 1 task with status %s, got %d", tc.status, len(result))
			}
			if len(result) > 0 && result[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s", tc.expectedStatus, result[0].Status)
			}
		})
	}
}

func TestFilterTasks_InvalidStatus(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true
	listTaskStatus = "invalid_status"

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress),
	}

	result := filterTasks(tasks)

	if len(result) != 0 {
		t.Errorf("expected 0 tasks with invalid status, got %d", len(result))
	}
}

func TestFilterTasks_ByStatusReviewAndStandby(t *testing.T) {
	defer resetFilterVars()
	listShowAll = true

	tasks := []*models.Task{
		testutil.NewTestTaskWithStatus("Todo task", models.StatusTodo),
		testutil.NewTestTaskWithStatus("Review task", models.StatusReview),
		testutil.NewTestTaskWithStatus("Standby task", models.StatusStandby),
	}

	listTaskStatus = "review"
	result := filterTasks(tasks)
	if len(result) != 1 {
		t.Errorf("expected 1 review task, got %d", len(result))
	}
	if len(result) > 0 && result[0].Status != models.StatusReview {
		t.Errorf("expected status review, got %s", result[0].Status)
	}

	listTaskStatus = "standby"
	result = filterTasks(tasks)
	if len(result) != 1 {
		t.Errorf("expected 1 standby task, got %d", len(result))
	}
	if len(result) > 0 && result[0].Status != models.StatusStandby {
		t.Errorf("expected status standby, got %s", result[0].Status)
	}
}
