package models

import (
	"testing"
	"time"
)

func TestGenerateRecurringInstancesStreakOnTime(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 15, 14, 30, 0, 0, time.Local)
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
	completedAt := time.Date(2025, 6, 17, 10, 0, 0, 0, time.Local)
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
	if newTask.Recurrence.Streak != 2 {
		t.Errorf("expected streak 2 (no due date = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 2 {
		t.Errorf("expected max streak 2, got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstancesStreakUpdatesMaxStreak(t *testing.T) {
	dueDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	completedAt := time.Date(2025, 6, 14, 10, 0, 0, 0, time.Local)
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

func TestGenerateRecurringInstances_NoDueDate_StreakIncrements(t *testing.T) {
	completedAt := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "rec-no-due",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring no due date",
		Status:      StatusDone,
		DueDate:     nil,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  2,
			Unit:      RecurrenceDaily,
			Streak:    3,
			MaxStreak: 5,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()

	if count != 1 {
		t.Fatalf("expected 1 instance generated, got %d", count)
	}
	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 4 {
		t.Errorf("expected streak 4 (nil due date = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 5 {
		t.Errorf("expected max streak 5 (unchanged), got %d", newTask.Recurrence.MaxStreak)
	}
	if newTask.DueDate == nil {
		t.Error("new task should have a DueDate calculated from today")
	}
}

func TestGenerateRecurringInstances_LateCompletion_StreakResets(t *testing.T) {
	dueDate := time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 8, 14, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "late-task",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Late recurring task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceWeekly,
			Streak:    10,
			MaxStreak: 15,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 0 {
		t.Errorf("expected streak 0 (late completion resets), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 15 {
		t.Errorf("expected max streak 15 (preserved from before), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstances_StreakSurpassesMaxStreak(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "streak-max",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Streak record task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceDaily,
			Streak:    9,
			MaxStreak: 9,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 10 {
		t.Errorf("expected streak 10 (incremented), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 10 {
		t.Errorf("expected max streak 10 (new record), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstances_CompletedExactlyOnDueDay(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 10, 23, 59, 59, 0, time.UTC)
	task := &Task{
		ID:          "exact-day",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Exact day completion",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceDaily,
			Streak:    0,
			MaxStreak: 0,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 1 {
		t.Errorf("expected streak 1 (same day = on-time), got %d", newTask.Recurrence.Streak)
	}
}

func TestGenerateRecurringInstances_EarlyCompletion_OnTime(t *testing.T) {
	dueDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 12, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "early-task",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Early completion",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceWeekly,
			Streak:    5,
			MaxStreak: 5,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 6 {
		t.Errorf("expected streak 6 (early = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 6 {
		t.Errorf("expected max streak 6 (new record), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstances_NilCompletedAt_OnTime(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "nil-completed",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Nil CompletedAt",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: nil,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceDaily,
			Streak:    2,
			MaxStreak: 2,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 3 {
		t.Errorf("expected streak 3 (nil CompletedAt = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 3 {
		t.Errorf("expected max streak 3, got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstances_OneDayLate_Resets(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 23, 59, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 11, 0, 1, 0, 0, time.UTC)
	task := &Task{
		ID:          "one-day-late",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Barely late",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceDaily,
			Streak:    5,
			MaxStreak: 10,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 0 {
		t.Errorf("expected streak 0 (one day late resets), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 10 {
		t.Errorf("expected max streak 10 (preserved), got %d", newTask.Recurrence.MaxStreak)
	}
}

func TestGenerateRecurringInstances_StreakFromZero(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 9, 15, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "zero-streak",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Fresh start",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  1,
			Unit:      RecurrenceDaily,
			Streak:    0,
			MaxStreak: 0,
		},
	}

	project := &Project{
		Name:        "Edge Case Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.Recurrence.Streak != 1 {
		t.Errorf("expected streak 1, got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 1 {
		t.Errorf("expected max streak 1, got %d", newTask.Recurrence.MaxStreak)
	}
}
