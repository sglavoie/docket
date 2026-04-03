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
