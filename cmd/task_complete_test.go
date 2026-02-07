package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestUpdateTaskStatusComplete(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	task := project.Tasks[0]
	originalStatus := task.Status

	// Verify the task starts in a non-complete state
	if task.Status == models.StatusDone {
		t.Skip("test task already done")
	}

	task.Complete()
	testutil.AssertTaskStatus(t, task, models.StatusDone)

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set after Complete()")
	}

	// Verify we can check original status
	if originalStatus == models.StatusDone {
		t.Error("original status should not have been done")
	}
}

func TestUpdateTaskStatusStart(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskStatus(t, task, models.StatusTodo)

	task.Start()
	testutil.AssertTaskStatus(t, task, models.StatusInProgress)
}

func TestUpdateTaskStatusBlock(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskStatus(t, task, models.StatusTodo)

	task.Block()
	testutil.AssertTaskStatus(t, task, models.StatusBlocked)
}

func TestUpdateTaskStatusCancel(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	testutil.AssertTaskStatus(t, task, models.StatusTodo)

	task.Cancel()
	testutil.AssertTaskStatus(t, task, models.StatusCanceled)

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set after Cancel()")
	}
}

func TestUpdateTaskStatusReopen(t *testing.T) {
	task := testutil.NewTestTaskWithStatus("Completed Task", models.StatusDone)
	testutil.AssertTaskStatus(t, task, models.StatusDone)

	if task.CompletedAt == nil {
		t.Error("completed task should have CompletedAt set")
	}

	task.Reopen()
	testutil.AssertTaskStatus(t, task, models.StatusTodo)

	if task.CompletedAt != nil {
		t.Error("CompletedAt should be cleared after Reopen()")
	}
}

func TestCompleteBlockedByDependencies(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	dependent := models.NewTask("dependent-1", 2, models.TaskTypeTask, "Dependent Task")
	dependent.AddDependency(blocker.ID)

	project.Tasks = []*models.Task{blocker, dependent}

	// Verify we have incomplete dependencies
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("dependent should have incomplete dependencies")
	}

	incompleteDeps := project.GetIncompleteDependencies(dependent)
	if len(incompleteDeps) != 1 {
		t.Errorf("expected 1 incomplete dependency, got %d", len(incompleteDeps))
	}
	if incompleteDeps[0].ID != blocker.ID {
		t.Errorf("expected blocker to be the incomplete dependency")
	}

	// Complete the blocker
	blocker.Complete()

	// Now the dependent should have no incomplete dependencies
	if project.HasIncompleteDependencies(dependent) {
		t.Error("dependent should not have incomplete dependencies after blocker is complete")
	}
}

func TestAutoUnblockAfterComplete(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	blocked := models.NewTask("blocked-1", 2, models.TaskTypeTask, "Blocked Task")
	blocked.AddDependency(blocker.ID)
	blocked.Status = models.StatusBlocked

	project.Tasks = []*models.Task{blocker, blocked}

	testutil.AssertTaskStatus(t, blocked, models.StatusBlocked)

	// Complete the blocker
	blocker.Complete()

	// Run auto-unblock check
	project.CheckAndAutoUnblock()

	// Blocked task should now be unblocked to todo
	testutil.AssertTaskStatus(t, blocked, models.StatusTodo)
}

func TestAutoUnblockAfterCancel(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	blocker := models.NewTask("blocker-1", 1, models.TaskTypeTask, "Blocker Task")
	blocked := models.NewTask("blocked-1", 2, models.TaskTypeTask, "Blocked Task")
	blocked.AddDependency(blocker.ID)
	blocked.Status = models.StatusBlocked

	project.Tasks = []*models.Task{blocker, blocked}

	// Cancel the blocker (also considered complete for dependency purposes)
	blocker.Cancel()

	// Run auto-unblock check
	project.CheckAndAutoUnblock()

	// Blocked task should now be unblocked to todo
	testutil.AssertTaskStatus(t, blocked, models.StatusTodo)
}

func TestStatusTransitions(t *testing.T) {
	tests := []struct {
		name         string
		initialState models.TaskStatus
		action       func(*models.Task)
		wantStatus   models.TaskStatus
	}{
		{
			name:         "todo to in_progress",
			initialState: models.StatusTodo,
			action:       (*models.Task).Start,
			wantStatus:   models.StatusInProgress,
		},
		{
			name:         "todo to done",
			initialState: models.StatusTodo,
			action:       (*models.Task).Complete,
			wantStatus:   models.StatusDone,
		},
		{
			name:         "todo to blocked",
			initialState: models.StatusTodo,
			action:       (*models.Task).Block,
			wantStatus:   models.StatusBlocked,
		},
		{
			name:         "todo to canceled",
			initialState: models.StatusTodo,
			action:       (*models.Task).Cancel,
			wantStatus:   models.StatusCanceled,
		},
		{
			name:         "in_progress to done",
			initialState: models.StatusInProgress,
			action:       (*models.Task).Complete,
			wantStatus:   models.StatusDone,
		},
		{
			name:         "in_progress to blocked",
			initialState: models.StatusInProgress,
			action:       (*models.Task).Block,
			wantStatus:   models.StatusBlocked,
		},
		{
			name:         "blocked to in_progress",
			initialState: models.StatusBlocked,
			action:       (*models.Task).Start,
			wantStatus:   models.StatusInProgress,
		},
		{
			name:         "done to todo via reopen",
			initialState: models.StatusDone,
			action:       (*models.Task).Reopen,
			wantStatus:   models.StatusTodo,
		},
		{
			name:         "canceled to todo via reopen",
			initialState: models.StatusCanceled,
			action:       (*models.Task).Reopen,
			wantStatus:   models.StatusTodo,
		},
		{
			name:         "in_progress to review",
			initialState: models.StatusInProgress,
			action:       (*models.Task).Review,
			wantStatus:   models.StatusReview,
		},
		{
			name:         "todo to review",
			initialState: models.StatusTodo,
			action:       (*models.Task).Review,
			wantStatus:   models.StatusReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := testutil.NewTestTaskWithStatus("Test Task", tt.initialState)
			tt.action(task)
			testutil.AssertTaskStatus(t, task, tt.wantStatus)
		})
	}
}

func TestStatusIsComplete(t *testing.T) {
	tests := []struct {
		status   models.TaskStatus
		complete bool
	}{
		{models.StatusTodo, false},
		{models.StatusInProgress, false},
		{models.StatusReview, false},
		{models.StatusBlocked, false},
		{models.StatusDone, true},
		{models.StatusCanceled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsComplete(); got != tt.complete {
				t.Errorf("TaskStatus(%q).IsComplete() = %v, want %v", tt.status, got, tt.complete)
			}
		})
	}
}

func TestStatusIsActive(t *testing.T) {
	tests := []struct {
		status models.TaskStatus
		active bool
	}{
		{models.StatusTodo, true},
		{models.StatusInProgress, true},
		{models.StatusReview, true},
		{models.StatusBlocked, true},
		{models.StatusDone, false},
		{models.StatusCanceled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsActive(); got != tt.active {
				t.Errorf("TaskStatus(%q).IsActive() = %v, want %v", tt.status, got, tt.active)
			}
		})
	}
}
