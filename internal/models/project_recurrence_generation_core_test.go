package models

import (
	"testing"
	"time"
)

func TestGenerateRecurringInstances(t *testing.T) {
	t.Run("completed recurring task generates new instance", func(t *testing.T) {
		dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		task := &Task{
			ID:            "task-1",
			ShortID:       1,
			Type:          TaskTypeTask,
			Title:         "Recurring Task",
			WorkflowState: StateDone,
			Priority:      2,
			Tags:          []string{"test", "recurring"},
			DueDate:       &dueDate,
			Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		}

		project := &Project{
			Name:        "Test Project",
			Tasks:       []*Task{task},
			NextShortID: 2,
		}

		count := project.GenerateRecurringInstances()

		if count != 1 {
			t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
		}
		if len(project.Tasks) != 2 {
			t.Fatalf("len(Tasks) = %d, want 2", len(project.Tasks))
		}

		newTask := project.Tasks[0]
		if newTask.State() != StateInbox {
			t.Errorf("new task State = %s, want inbox", newTask.State())
		}
		if newTask.Title != "Recurring Task" {
			t.Errorf("new task Title = %s, want 'Recurring Task'", newTask.Title)
		}
		if newTask.Priority != 2 {
			t.Errorf("new task Priority = %d, want 2", newTask.Priority)
		}
		if len(newTask.Tags) != 2 || newTask.Tags[0] != "test" || newTask.Tags[1] != "recurring" {
			t.Errorf("new task Tags = %v, want [test recurring]", newTask.Tags)
		}
		if newTask.Recurrence == nil {
			t.Error("new task Recurrence should not be nil")
		}
		expectedDue := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)
		if newTask.DueDate == nil || !newTask.DueDate.Equal(expectedDue) {
			t.Errorf("new task DueDate = %v, want %v", newTask.DueDate, expectedDue)
		}

		originalTask := project.Tasks[1]
		if originalTask.Recurrence != nil {
			t.Error("original task Recurrence should be nil after generation")
		}
	})

	t.Run("non-recurring completed task does not generate", func(t *testing.T) {
		task := &Task{
			ID:            "task-1",
			WorkflowState: StateDone,
		}

		project := &Project{
			Tasks: []*Task{task},
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
		if len(project.Tasks) != 1 {
			t.Errorf("len(Tasks) = %d, want 1", len(project.Tasks))
		}
	})

	t.Run("recurring but not completed does not generate", func(t *testing.T) {
		dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		task := &Task{
			ID:            "task-1",
			WorkflowState: StateReady,
			DueDate:       &dueDate,
			Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}

		project := &Project{
			Tasks: []*Task{task},
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
		if len(project.Tasks) != 1 {
			t.Errorf("len(Tasks) = %d, want 1", len(project.Tasks))
		}
		if task.Recurrence == nil {
			t.Error("task Recurrence should still be set")
		}
	})

	t.Run("multiple recurring tasks completed", func(t *testing.T) {
		due1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		due2 := time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)

		task1 := &Task{
			ID:            "task-1",
			ShortID:       1,
			Title:         "Task 1",
			WorkflowState: StateDone,
			DueDate:       &due1,
			Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		task2 := &Task{
			ID:            "task-2",
			ShortID:       2,
			Title:         "Task 2",
			WorkflowState: StateDone,
			DueDate:       &due2,
			Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
		}
		task3 := &Task{
			ID:            "task-3",
			ShortID:       3,
			Title:         "Task 3",
			WorkflowState: StateDone,
		}

		project := &Project{
			Tasks:       []*Task{task1, task2, task3},
			NextShortID: 4,
		}

		count := project.GenerateRecurringInstances()

		if count != 2 {
			t.Errorf("GenerateRecurringInstances() = %d, want 2", count)
		}
		if len(project.Tasks) != 5 {
			t.Errorf("len(Tasks) = %d, want 5", len(project.Tasks))
		}
	})
}

func TestGenerateRecurringInstancesNoDueDate(t *testing.T) {
	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Title:         "No Due Date Recurring",
		WorkflowState: StateDone,
		DueDate:       nil,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()

	if count != 1 {
		t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
	}
	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	if newTask.DueDate == nil {
		t.Error("new task DueDate should not be nil - should be calculated from today")
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	if newTask.DueDate != nil {
		gotDay := newTask.DueDate.Truncate(24 * time.Hour)
		wantDay := tomorrow.Truncate(24 * time.Hour)
		if !gotDay.Equal(wantDay) {
			t.Errorf("new task DueDate = %v, want tomorrow (%v)", gotDay, wantDay)
		}
	}
	if newTask.Recurrence == nil {
		t.Error("new task should still have recurrence pattern")
	}
}

func TestGenerateRecurringInstancesIdempotency(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Title:         "Recurring Task",
		WorkflowState: StateDone,
		DueDate:       &dueDate,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count1 := project.GenerateRecurringInstances()
	if count1 != 1 {
		t.Errorf("first call: count = %d, want 1", count1)
	}

	count2 := project.GenerateRecurringInstances()
	if count2 != 0 {
		t.Errorf("second call: count = %d, want 0 (idempotent)", count2)
	}

	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2 after both calls", len(project.Tasks))
	}
}

func TestGenerateRecurringInstancesNonDoneStatuses(t *testing.T) {
	states := []WorkflowState{StateReady, StateInProgress, StateWaiting, StateCanceled}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			task := &Task{
				ID:            "task-1",
				ShortID:       1,
				Title:         "Non-done recurring",
				WorkflowState: state,
				DueDate:       &dueDate,
				Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			}

			project := &Project{
				Tasks:       []*Task{task},
				NextShortID: 2,
			}

			count := project.GenerateRecurringInstances()

			if count != 0 {
				t.Errorf("GenerateRecurringInstances() with state %s = %d, want 0", state, count)
			}
			if len(project.Tasks) != 1 {
				t.Errorf("len(Tasks) = %d, want 1 (no new task)", len(project.Tasks))
			}
			if task.Recurrence == nil {
				t.Errorf("recurrence should NOT be cleared for non-done task")
			}
		})
	}
}

func TestGenerateRecurringInstances_NonRecurringCompleted_Skipped(t *testing.T) {
	completedAt := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	task := &Task{
		ID:            "non-rec",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Normal completed task",
		WorkflowState: StateDone,
		CompletedAt:   &completedAt,
		Recurrence:    nil,
		Tags:          []string{"important"},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()

	if count != 0 {
		t.Errorf("expected 0 instances generated for non-recurring task, got %d", count)
	}
	if len(project.Tasks) != 1 {
		t.Errorf("expected 1 task (unchanged), got %d", len(project.Tasks))
	}
	if project.Tasks[0].Title != "Normal completed task" {
		t.Error("original task should not be modified")
	}
	if project.Tasks[0].State() != StateDone {
		t.Error("original task state should remain done")
	}
}

func TestGenerateRecurringInstancesPreservesTaskType(t *testing.T) {
	types := []TaskType{TaskTypeBug, TaskTypeIdea, TaskTypeEpic, TaskTypeStory}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, taskType := range types {
		t.Run(string(taskType), func(t *testing.T) {
			task := &Task{
				ID:            "task-1",
				ShortID:       1,
				Type:          taskType,
				Title:         "Recurring " + string(taskType),
				WorkflowState: StateDone,
				DueDate:       &dueDate,
				Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			}

			project := &Project{
				Tasks:       []*Task{task},
				NextShortID: 2,
			}

			project.GenerateRecurringInstances()

			if len(project.Tasks) != 2 {
				t.Fatalf("len(Tasks) = %d, want 2", len(project.Tasks))
			}

			newTask := project.Tasks[0]
			if newTask.Type != taskType {
				t.Errorf("new task Type = %s, want %s", newTask.Type, taskType)
			}
		})
	}
}

func TestGenerateRecurringInstancesEmptyProject(t *testing.T) {
	t.Run("nil tasks slice", func(t *testing.T) {
		project := &Project{
			Tasks:       nil,
			NextShortID: 1,
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
	})

	t.Run("empty tasks slice", func(t *testing.T) {
		project := &Project{
			Tasks:       []*Task{},
			NextShortID: 1,
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
	})
}

func TestGenerateRecurringInstancesNewIDIsValidUUID(t *testing.T) {
	// Verify that GenerateRecurringInstances assigns a proper RFC 4122 UUID to
	// the new instance (not the old non-standard timestamp-based format).
	dueDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:            "parent-id",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Recurring UUID test",
		WorkflowState: StateDone,
		DueDate:       &dueDate,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	if len(project.Tasks) < 1 {
		t.Fatal("expected at least one task after generation")
	}
	newTask := project.Tasks[0]
	if newTask.ID == "" {
		t.Fatal("expected new task to have a non-empty ID")
	}
	if newTask.ID == task.ID {
		t.Errorf("new task must not reuse the completed task's ID, got %q", newTask.ID)
	}
	// UUID v4 format: 8-4-4-4-12 hex chars with hyphens
	// len("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") == 36
	if len(newTask.ID) != 36 {
		t.Errorf("expected standard UUID (36 chars), got %q (len %d)", newTask.ID, len(newTask.ID))
	}
}
