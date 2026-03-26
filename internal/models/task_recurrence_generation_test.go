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
			"no due date but has recurrence",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			nil,
			false, // Now calculates from today
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
	// Recurring task without due date should generate with due date based on today
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

	if count != 1 {
		t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
	}
	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	// When original has no due date, new instance should get due date based on today
	if newTask.DueDate == nil {
		t.Error("new task DueDate should not be nil - should be calculated from today")
	}
	// Verify it's tomorrow (daily recurrence from today)
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

func TestGenerateRecurringInstancesCopiesCorrectFields(t *testing.T) {
	parentID := "parent-1"
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring Task",
		Description: "Weekly standup agenda",
		Status:      StatusDone,
		Priority:    2,
		Section:     SectionNext,
		Tags:        []string{"test"},
		DueDate:     &dueDate,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1", "dep-2"},
		Stage:       "Review",
		Checklist: []ChecklistItem{
			{Text: "Item 1", Done: true},
			{Text: "Item 2", Done: false},
		},
		ScoreDimensions: map[string]int{"impact": 4, "effort": 2},
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
	if newTask.Description != "Weekly standup agenda" {
		t.Errorf("Description should be copied, got %q", newTask.Description)
	}
	if newTask.Stage != "Review" {
		t.Errorf("Stage should be copied, got %q", newTask.Stage)
	}
	if len(newTask.ScoreDimensions) != 2 || newTask.ScoreDimensions["impact"] != 4 || newTask.ScoreDimensions["effort"] != 2 {
		t.Errorf("ScoreDimensions should be copied, got %v", newTask.ScoreDimensions)
	}

	// Checklist should be copied but reset to unchecked
	if len(newTask.Checklist) != 2 {
		t.Errorf("Checklist should be copied, got %d items", len(newTask.Checklist))
	} else {
		if newTask.Checklist[0].Text != "Item 1" || newTask.Checklist[0].Done {
			t.Errorf("Checklist item 0 should be unchecked copy, got %+v", newTask.Checklist[0])
		}
		if newTask.Checklist[1].Text != "Item 2" || newTask.Checklist[1].Done {
			t.Errorf("Checklist item 1 should be unchecked copy, got %+v", newTask.Checklist[1])
		}
	}

	// Fields that should NOT be copied (relationship/context-specific)
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

func TestGenerateRecurringInstancesStreakOnTime(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 15, 14, 30, 0, 0, time.Local) // Same day = on time
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Title:       "Streak task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceWeekly,
			Streak:    2,
			MaxStreak: 5,
		},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 3 {
		t.Errorf("expected streak 3 (on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 5 {
		t.Errorf("expected max streak 5 (unchanged), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstancesStreakLate(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 17, 10, 0, 0, 0, time.Local) // 2 days late
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Title:       "Late task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceWeekly,
			Streak:    4,
			MaxStreak: 4,
		},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 0 {
		t.Errorf("expected streak 0 (late completion resets), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 4 {
		t.Errorf("expected max streak 4 (preserved), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstancesStreakNoDueDate(t *testing.T) {
	completedAt := time.Date(2025, 6, 15, 14, 0, 0, 0, time.Local)
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Title:       "No due date task",
		Status:      StatusDone,
		DueDate:     nil,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval: 1,
			Unit:     RecurrenceDaily,
			Streak:   1,
		},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	// No due date counts as on-time
	if newTask.Recurrence.Streak != 2 {
		t.Errorf("expected streak 2 (no due date = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 2 {
		t.Errorf("expected max streak 2, got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstancesStreakUpdatesMaxStreak(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 14, 10, 0, 0, 0, time.Local) // Early = on time
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Title:       "Max streak task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceWeekly,
			Streak:    7,
			MaxStreak: 7,
		},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 8 {
		t.Errorf("expected streak 8, got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 8 {
		t.Errorf("expected max streak 8 (new record), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstancesSkippedCarriedForward(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 15, 10, 0, 0, 0, time.Local)
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Title:       "Skipped counter task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval: 1,
			Unit:     RecurrenceWeekly,
			Skipped:  3,
		},
	}

	project := &Project{
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Skipped != 3 {
		t.Errorf("expected skipped 3 (carried forward), got %d", newTask.Recurrence.Skipped)
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

func TestGenerateRecurringInstancesCopiesTypeAttrs(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("BugAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-1",
			ShortID:    1,
			Type:       TaskTypeBug,
			Title:      "Recurring Bug",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			BugAttrs: &BugAttrs{
				Severity:     BugSeverityCritical,
				Reproducible: BugReproducibleYes,
				Workaround:   "restart the server",
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.BugAttrs == nil {
			t.Fatal("BugAttrs should be copied to new instance")
		}
		if newTask.BugAttrs.Severity != BugSeverityCritical {
			t.Errorf("Severity = %s, want critical", newTask.BugAttrs.Severity)
		}
		if newTask.BugAttrs.Reproducible != BugReproducibleYes {
			t.Errorf("Reproducible = %s, want yes", newTask.BugAttrs.Reproducible)
		}
		if newTask.BugAttrs.Workaround != "restart the server" {
			t.Errorf("Workaround = %q, want 'restart the server'", newTask.BugAttrs.Workaround)
		}
		// Verify deep copy (modifying original should not affect new)
		task.BugAttrs.Severity = BugSeverityMinor
		if newTask.BugAttrs.Severity != BugSeverityCritical {
			t.Error("BugAttrs should be a deep copy, not a shared pointer")
		}
	})

	t.Run("TaskAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-2",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring Task",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			TaskAttrs: &TaskAttrs{
				Effort:   EffortSmall,
				WorkType: WorkTypeDevelopment,
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.TaskAttrs == nil {
			t.Fatal("TaskAttrs should be copied to new instance")
		}
		if newTask.TaskAttrs.Effort != EffortSmall {
			t.Errorf("Effort = %s, want small", newTask.TaskAttrs.Effort)
		}
		if newTask.TaskAttrs.WorkType != WorkTypeDevelopment {
			t.Errorf("WorkType = %s, want development", newTask.TaskAttrs.WorkType)
		}
	})

	t.Run("IdeaAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-3",
			ShortID:    1,
			Type:       TaskTypeIdea,
			Title:      "Recurring Idea",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
			IdeaAttrs: &IdeaAttrs{
				Category:  IdeaCategoryFeature,
				Effort:    EffortLarge,
				Value:     IdeaValueHigh,
				Validated: true,
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.IdeaAttrs == nil {
			t.Fatal("IdeaAttrs should be copied to new instance")
		}
		if newTask.IdeaAttrs.Category != IdeaCategoryFeature {
			t.Errorf("Category = %s, want feature", newTask.IdeaAttrs.Category)
		}
		if !newTask.IdeaAttrs.Validated {
			t.Error("Validated should be true")
		}
	})

	t.Run("Source and URL copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-4",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring with metadata",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			Source:     "github",
			URL:        "https://github.com/example/issue/42",
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.Source != "github" {
			t.Errorf("Source = %q, want 'github'", newTask.Source)
		}
		if newTask.URL != "https://github.com/example/issue/42" {
			t.Errorf("URL = %q, want 'https://github.com/example/issue/42'", newTask.URL)
		}
	})

	t.Run("nil attrs not copied", func(t *testing.T) {
		task := &Task{
			ID:         "task-5",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Plain recurring",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.BugAttrs != nil {
			t.Error("BugAttrs should be nil for task without bug attrs")
		}
		if newTask.TaskAttrs != nil {
			t.Error("TaskAttrs should be nil for task without task attrs")
		}
	})
}
