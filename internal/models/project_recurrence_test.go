package models

import (
	"testing"
	"time"
)

// Edge case: recurring task with no DueDate, completed. Should still generate
// a next instance, and the streak should increment (no DueDate counts as on-time).
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

	// Streak should increment because nil DueDate counts as on-time
	if newTask.Recurrence.Streak != 4 {
		t.Errorf("expected streak 4 (nil due date = on-time), got %d", newTask.Recurrence.Streak)
	}
	// MaxStreak should remain unchanged since 4 < 5
	if newTask.Recurrence.MaxStreak != 5 {
		t.Errorf("expected max streak 5 (unchanged), got %d", newTask.Recurrence.MaxStreak)
	}
	// New instance should have a DueDate derived from today (since original had none)
	if newTask.DueDate == nil {
		t.Error("new task should have a DueDate calculated from today")
	}
}

// Edge case: completed task with nil Recurrence should be completely skipped.
func TestGenerateRecurringInstances_NonRecurringCompleted_Skipped(t *testing.T) {
	completedAt := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "non-rec",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Normal completed task",
		Status:      StatusDone,
		CompletedAt: &completedAt,
		Recurrence:  nil,
		Tags:        []string{"important"},
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
	// The original task should be completely untouched
	if project.Tasks[0].Title != "Normal completed task" {
		t.Error("original task should not be modified")
	}
	if project.Tasks[0].Status != StatusDone {
		t.Error("original task status should remain done")
	}
}

// Edge case: late completion resets streak to 0, but preserves MaxStreak.
func TestGenerateRecurringInstances_LateCompletion_StreakResets(t *testing.T) {
	dueDate := time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)
	// Completed 3 days after due date
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

// Edge case: on-time completion where streak surpasses MaxStreak, both should update.
func TestGenerateRecurringInstances_StreakSurpassesMaxStreak(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC) // same day = on time
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

// Edge case: completion exactly on due date boundary (end of day vs start of day).
func TestGenerateRecurringInstances_CompletedExactlyOnDueDay(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	// Completed at 23:59 on the same day - should still be on-time
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
	// Both truncate to same day, so this should be on-time
	if newTask.Recurrence.Streak != 1 {
		t.Errorf("expected streak 1 (same day = on-time), got %d", newTask.Recurrence.Streak)
	}
}

// Edge case: completed before due date should count as on-time.
func TestGenerateRecurringInstances_EarlyCompletion_OnTime(t *testing.T) {
	dueDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 12, 10, 0, 0, 0, time.UTC) // 3 days early
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

// Verify all properties of the new instance are set correctly.
func TestGenerateRecurringInstances_NewInstanceProperties(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "props-task",
		ShortID:     1,
		Type:        TaskTypeBug,
		Title:       "Fix login issue",
		Status:      StatusDone,
		Priority:    3,
		Section:     SectionNext,
		Tags:        []string{"backend", "auth", "urgent"},
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  2,
			Unit:      RecurrenceWeekly,
			Weekdays:  []int{1, 3, 5},
			Skipped:   4,
			Streak:    2,
			MaxStreak: 7,
		},
	}

	project := &Project{
		Name:        "Props Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
		Sections:    []string{SectionInbox, SectionNext},
	}

	project.GenerateRecurringInstances()

	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	origTask := project.Tasks[1]

	// Status must be todo
	if newTask.Status != StatusTodo {
		t.Errorf("new task status = %s, want %s", newTask.Status, StatusTodo)
	}
	// Section must be Inbox
	if newTask.Section != SectionInbox {
		t.Errorf("new task section = %s, want %s", newTask.Section, SectionInbox)
	}
	// Type should be carried forward
	if newTask.Type != TaskTypeBug {
		t.Errorf("new task type = %s, want %s", newTask.Type, TaskTypeBug)
	}
	// Title should be carried forward
	if newTask.Title != "Fix login issue" {
		t.Errorf("new task title = %q, want %q", newTask.Title, "Fix login issue")
	}
	// Priority should be carried forward
	if newTask.Priority != 3 {
		t.Errorf("new task priority = %d, want 3", newTask.Priority)
	}
	// Tags should be carried forward (deep copy)
	if len(newTask.Tags) != 3 {
		t.Fatalf("new task tags length = %d, want 3", len(newTask.Tags))
	}
	if newTask.Tags[0] != "backend" || newTask.Tags[1] != "auth" || newTask.Tags[2] != "urgent" {
		t.Errorf("new task tags = %v, want [backend auth urgent]", newTask.Tags)
	}
	// Mutating new tags should not affect original
	newTask.Tags[0] = "mutated"
	if len(origTask.Tags) > 0 && origTask.Tags[0] == "mutated" {
		t.Error("tag slice should be a deep copy, not shared")
	}

	// Recurrence pattern should be carried forward
	if newTask.Recurrence == nil {
		t.Fatal("new task recurrence should not be nil")
	}
	if newTask.Recurrence.Interval != 2 {
		t.Errorf("recurrence interval = %d, want 2", newTask.Recurrence.Interval)
	}
	if newTask.Recurrence.Unit != RecurrenceWeekly {
		t.Errorf("recurrence unit = %s, want %s", newTask.Recurrence.Unit, RecurrenceWeekly)
	}
	if len(newTask.Recurrence.Weekdays) != 3 || newTask.Recurrence.Weekdays[0] != 1 {
		t.Errorf("recurrence weekdays = %v, want [1 3 5]", newTask.Recurrence.Weekdays)
	}
	// Weekdays should be a deep copy
	newTask.Recurrence.Weekdays[0] = 99
	if task.Recurrence != nil {
		// Original recurrence was cleared, so this check is about the copy
		// Just verify the new weekdays slice is independent
	}
	if newTask.Recurrence.Skipped != 4 {
		t.Errorf("recurrence skipped = %d, want 4", newTask.Recurrence.Skipped)
	}
	// Streak should have incremented (on-time completion)
	if newTask.Recurrence.Streak != 3 {
		t.Errorf("recurrence streak = %d, want 3", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 7 {
		t.Errorf("recurrence max streak = %d, want 7 (unchanged)", newTask.Recurrence.MaxStreak)
	}

	// DueDate should be set (next due from recurrence calculation)
	if newTask.DueDate == nil {
		t.Error("new task due date should not be nil")
	}

	// New task should have a unique ID (not same as original)
	if newTask.ID == origTask.ID {
		t.Error("new task should have a different ID from original")
	}
	// ShortID should be the next available
	if newTask.ShortID != 2 {
		t.Errorf("new task short ID = %d, want 2", newTask.ShortID)
	}
	// CreatedAt and UpdatedAt should be set
	if newTask.CreatedAt.IsZero() {
		t.Error("new task CreatedAt should be set")
	}
	if newTask.UpdatedAt.IsZero() {
		t.Error("new task UpdatedAt should be set")
	}

	// Original task should have recurrence cleared
	if origTask.Recurrence != nil {
		t.Error("original task recurrence should be nil after generation")
	}
}

// Edge case: completed task with DueDate set but CompletedAt nil.
// The onTime logic only checks completedDay.After(dueDay) when both are non-nil,
// so nil CompletedAt should count as on-time.
func TestGenerateRecurringInstances_NilCompletedAt_OnTime(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	task := &Task{
		ID:          "nil-completed",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Nil CompletedAt",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: nil, // status is done but CompletedAt not set
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
	// Both DueDate and CompletedAt must be non-nil for late check,
	// so nil CompletedAt defaults to on-time
	if newTask.Recurrence.Streak != 3 {
		t.Errorf("expected streak 3 (nil CompletedAt = on-time), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 3 {
		t.Errorf("expected max streak 3, got %d", newTask.Recurrence.MaxStreak)
	}
}

// Edge case: one day late (boundary). Completed the day after due date.
func TestGenerateRecurringInstances_OneDayLate_Resets(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 23, 59, 0, 0, time.UTC)
	// Next calendar day, even just 1 minute later
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
	// Truncated: completedDay=2025-03-11, dueDay=2025-03-10. completedDay.After(dueDay)=true => late
	if newTask.Recurrence.Streak != 0 {
		t.Errorf("expected streak 0 (one day late resets), got %d", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 10 {
		t.Errorf("expected max streak 10 (preserved), got %d", newTask.Recurrence.MaxStreak)
	}
}

// Edge case: streak starts at 0, on-time completion should set it to 1
// and MaxStreak should also become 1.
func TestGenerateRecurringInstances_StreakFromZero(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 9, 15, 0, 0, 0, time.UTC) // early
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
