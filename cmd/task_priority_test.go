package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSetTaskPriority(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)
	originalPriority := task.Priority
	originalUpdatedAt := task.UpdatedAt

	// Default priority should be 3
	testutil.AssertEqual(t, task.Priority, 3)

	// Simulate priority change
	task.Priority = 1
	task.UpdatedAt = time.Now()

	testutil.AssertEqual(t, task.Priority, 1)
	if !task.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after priority change")
	}

	// Verify original was different
	if originalPriority == 1 {
		t.Error("original priority should not have been 1")
	}
}

func TestSetTaskPriorityValidation(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		valid    bool
	}{
		{"valid priority 1", 1, true},
		{"valid priority 2", 2, true},
		{"valid priority 3", 3, true},
		{"valid priority 4", 4, true},
		{"valid priority 5", 5, true},
		{"invalid priority 0", 0, false},
		{"invalid priority 6", 6, false},
		{"invalid priority negative", -1, false},
		{"invalid priority large", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.priority >= 1 && tt.priority <= 5
			testutil.AssertEqual(t, valid, tt.valid)
		})
	}
}

func TestSetTaskPriorityWithTestutil(t *testing.T) {
	// Test NewTestTaskWithPriority helper
	task := testutil.NewTestTaskWithPriority("High Priority Task", 1)
	testutil.AssertEqual(t, task.Priority, 1)

	task2 := testutil.NewTestTaskWithPriority("Low Priority Task", 5)
	testutil.AssertEqual(t, task2.Priority, 5)
}

func TestTaskPriorityInProject(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")

	// Find the epic which has priority 1
	var epicTask *models.Task
	for _, task := range project.Tasks {
		if task.Type == models.TaskTypeEpic {
			epicTask = task
			break
		}
	}

	if epicTask == nil {
		t.Fatal("expected to find an epic task")
	}

	testutil.AssertEqual(t, epicTask.Priority, 1)

	// Change priority
	oldPriority := epicTask.Priority
	epicTask.Priority = 3
	epicTask.UpdatedAt = time.Now()

	testutil.AssertEqual(t, epicTask.Priority, 3)
	if oldPriority == 3 {
		t.Error("old priority should not have been 3")
	}
}

func TestTaskPriorityLookupByShortID(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")

	// Get task by short ID
	task := project.GetTaskByShortID(1)
	if task == nil {
		t.Fatal("expected to find task with short ID 1")
	}

	originalPriority := task.Priority
	task.Priority = 2

	testutil.AssertEqual(t, task.Priority, 2)
	if originalPriority == task.Priority {
		t.Error("priority should have changed")
	}
}

func TestTaskPriorityNotFound(t *testing.T) {
	project := testutil.NewTestProject("Empty Project")

	// Try to get a non-existent task
	task := project.GetTaskByShortID(999)
	if task != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestPriorityNoChangeWhenSame(t *testing.T) {
	task := testutil.NewTestTaskWithPriority("Test Task", 2)
	originalUpdatedAt := task.UpdatedAt

	// Setting same priority shouldn't need to trigger an update
	// (the command handles this, but we test the concept)
	newPriority := 2
	if task.Priority == newPriority {
		// No change needed - this is the expected behavior
		testutil.AssertEqual(t, task.Priority, 2)
		testutil.AssertEqual(t, task.UpdatedAt, originalUpdatedAt)
	}
}

// TestTaskPriorityNoErrorOnNoOp confirms that running `tt task priority <id> N`
// against a task that already has priority N exits with no error (a no-op).
// Previously, the single-task path appended the "already at priority" sentinel
// to errs and ReportBatchErrors surfaced it as a non-zero exit.
func TestTaskPriorityNoErrorOnNoOp(t *testing.T) {
	deps, project := setupSectionCommandTest(t)

	task := project.Tasks[0]
	task.Priority = 2
	deps.Repository.Projects[project.ID] = project

	originalUpdatedAt := task.UpdatedAt

	err := runTaskPriority(nil, []string{fmt.Sprintf("%d", task.ShortID), "2"})
	if err != nil {
		t.Fatalf("runTaskPriority should not error when priority is already at target: %v", err)
	}

	savedProject := deps.Repository.Projects[project.ID]
	savedTask := savedProject.GetTaskByShortID(task.ShortID)
	if savedTask.UpdatedAt != originalUpdatedAt {
		t.Error("UpdatedAt should not change when priority is already at target")
	}
	if savedTask.Priority != 2 {
		t.Errorf("priority = %d, want 2", savedTask.Priority)
	}
}
