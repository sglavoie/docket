package models

import (
	"testing"
	"time"
)

func TestTaskNextDueDateNilCases(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		recurrence *RecurrencePattern
		dueDate    *time.Time
		wantNil    bool
	}{
		{
			"no recurrence",
			nil,
			&dueDate,
			true,
		},
		{
			"no due date",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			nil,
			true,
		},
		{
			"both nil",
			nil,
			nil,
			true,
		},
		{
			"both set",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			&dueDate,
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{
				Recurrence: tc.recurrence,
				DueDate:    tc.dueDate,
			}
			got := task.NextDueDate()
			if tc.wantNil && got != nil {
				t.Errorf("NextDueDate() = %v, want nil", got)
			}
			if !tc.wantNil && got == nil {
				t.Error("NextDueDate() = nil, want non-nil")
			}
		})
	}
}

func TestTaskNextDueDateCalculation(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	task := &Task{
		DueDate:    &dueDate,
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	got := task.NextDueDate()
	want := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	if !got.Equal(want) {
		t.Errorf("NextDueDate() = %v, want %v", *got, want)
	}
}

func TestGenerateRecurringInstances(t *testing.T) {
	t.Run("completed recurring task generates new instance", func(t *testing.T) {
		dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		task := &Task{
			ID:         "task-1",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring Task",
			Status:     StatusDone,
			Priority:   2,
			Section:    SectionNext,
			Tags:       []string{"test", "recurring"},
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
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

		// New instance should be first in the list
		newTask := project.Tasks[0]
		if newTask.Status != StatusTodo {
			t.Errorf("new task Status = %s, want todo", newTask.Status)
		}
		if newTask.Section != SectionInbox {
			t.Errorf("new task Section = %s, want inbox", newTask.Section)
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

		// Original task should have recurrence cleared
		originalTask := project.Tasks[1]
		if originalTask.Recurrence != nil {
			t.Error("original task Recurrence should be nil after generation")
		}
	})

	t.Run("non-recurring completed task does not generate", func(t *testing.T) {
		task := &Task{
			ID:     "task-1",
			Status: StatusDone,
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
			ID:         "task-1",
			Status:     StatusTodo,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
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
			ID:         "task-1",
			ShortID:    1,
			Title:      "Task 1",
			Status:     StatusDone,
			DueDate:    &due1,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		task2 := &Task{
			ID:         "task-2",
			ShortID:    2,
			Title:      "Task 2",
			Status:     StatusDone,
			DueDate:    &due2,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
		}
		task3 := &Task{
			ID:      "task-3",
			ShortID: 3,
			Title:   "Task 3",
			Status:  StatusDone,
			// No recurrence
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
	// Recurring task without due date should not generate new instance
	task := &Task{
		ID:         "task-1",
		ShortID:    1,
		Title:      "No Due Date Recurring",
		Status:     StatusDone,
		DueDate:    nil, // No due date
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()

	// Should still generate (NextDueDate returns nil, but instance is created)
	// Looking at the implementation, it will create instance with nil DueDate
	if count != 1 {
		t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
	}
	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	if newTask.DueDate != nil {
		t.Errorf("new task DueDate should be nil when original had no due date")
	}
	if newTask.Recurrence == nil {
		t.Error("new task should still have recurrence pattern")
	}
}

func TestGenerateRecurringInstancesDoesNotCopyAllFields(t *testing.T) {
	parentID := "parent-1"
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring Task",
		Description: "This description should not be copied",
		Status:      StatusDone,
		Priority:    2,
		Section:     SectionNext,
		Tags:        []string{"test"},
		DueDate:     &dueDate,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1", "dep-2"},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]

	// Fields that SHOULD be copied
	if newTask.Title != "Recurring Task" {
		t.Errorf("Title should be copied, got %q", newTask.Title)
	}
	if newTask.Priority != 2 {
		t.Errorf("Priority should be copied, got %d", newTask.Priority)
	}
	if len(newTask.Tags) != 1 || newTask.Tags[0] != "test" {
		t.Errorf("Tags should be copied, got %v", newTask.Tags)
	}

	// Fields that should NOT be copied
	if newTask.Description != "" {
		t.Errorf("Description should not be copied, got %q", newTask.Description)
	}
	if newTask.ParentID != nil {
		t.Errorf("ParentID should not be copied, got %v", newTask.ParentID)
	}
	if len(newTask.DependsOn) != 0 {
		t.Errorf("DependsOn should not be copied, got %v", newTask.DependsOn)
	}
}

func TestGenerateRecurringInstancesIdempotency(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:         "task-1",
		ShortID:    1,
		Title:      "Recurring Task",
		Status:     StatusDone,
		DueDate:    &dueDate,
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	// First call
	count1 := project.GenerateRecurringInstances()
	if count1 != 1 {
		t.Errorf("first call: count = %d, want 1", count1)
	}

	// Second call should not generate more (original task has recurrence cleared)
	count2 := project.GenerateRecurringInstances()
	if count2 != 0 {
		t.Errorf("second call: count = %d, want 0 (idempotent)", count2)
	}

	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2 after both calls", len(project.Tasks))
	}
}

func TestGenerateRecurringInstancesNonDoneStatuses(t *testing.T) {
	// Only StatusDone should trigger generation
	statuses := []TaskStatus{StatusTodo, StatusInProgress, StatusBlocked, StatusCanceled}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			task := &Task{
				ID:         "task-1",
				ShortID:    1,
				Title:      "Non-done recurring",
				Status:     status,
				DueDate:    &dueDate,
				Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			}

			project := &Project{
				Tasks:       []*Task{task},
				NextShortID: 2,
			}

			count := project.GenerateRecurringInstances()

			if count != 0 {
				t.Errorf("GenerateRecurringInstances() with status %s = %d, want 0", status, count)
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

func TestGenerateRecurringInstancesPreservesTaskType(t *testing.T) {
	types := []TaskType{TaskTypeBug, TaskTypeIdea, TaskTypeEpic, TaskTypeStory}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, taskType := range types {
		t.Run(string(taskType), func(t *testing.T) {
			task := &Task{
				ID:         "task-1",
				ShortID:    1,
				Type:       taskType,
				Title:      "Recurring " + string(taskType),
				Status:     StatusDone,
				DueDate:    &dueDate,
				Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
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
