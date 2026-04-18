package models

import (
	"testing"
	"time"
)

// TestNextDueDate_FromDue verifies that from-due mode (default) uses the due
// date as base, even when CompletedAt is set.
func TestNextDueDate_FromDue(t *testing.T) {
	dueDate := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2024, 3, 10, 14, 0, 0, 0, time.UTC)
	task := &Task{
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence:  &RecurrencePattern{Interval: 3, Unit: RecurrenceDaily},
	}

	got := task.NextDueDate()
	want := time.Date(2024, 3, 13, 0, 0, 0, 0, time.UTC) // due + 3d

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	if !got.Equal(want) {
		t.Errorf("NextDueDate() = %v, want %v", *got, want)
	}
}

// TestNextDueDate_FromCompletion_OnTime verifies that on-time completion uses
// CompletedAt as base when Mode == RecurModeFromCompletion.
func TestNextDueDate_FromCompletion_OnTime(t *testing.T) {
	dueDate := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2024, 3, 10, 14, 0, 0, 0, time.UTC) // same day
	task := &Task{
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence:  &RecurrencePattern{Interval: 3, Unit: RecurrenceDaily, Mode: RecurModeFromCompletion},
	}

	got := task.NextDueDate()
	want := completedAt.AddDate(0, 0, 3) // completedAt + 3d

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	if !got.Equal(want) {
		t.Errorf("NextDueDate() = %v, want %v", *got, want)
	}
}

// TestNextDueDate_FromCompletion_Late verifies that a late completion (after
// due) uses CompletedAt — not DueDate — as the base (Q3).
func TestNextDueDate_FromCompletion_Late(t *testing.T) {
	dueDate := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2024, 3, 15, 9, 0, 0, 0, time.UTC) // 5 days late
	task := &Task{
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence:  &RecurrencePattern{Interval: 3, Unit: RecurrenceDaily, Mode: RecurModeFromCompletion},
	}

	got := task.NextDueDate()
	want := completedAt.AddDate(0, 0, 3) // late-completion + 3d

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	if !got.Equal(want) {
		t.Errorf("NextDueDate() = %v, want %v (should be based on late completion)", *got, want)
	}
	// Explicitly assert it is NOT based on due date
	notWant := dueDate.AddDate(0, 0, 3)
	if got.Equal(notWant) {
		t.Errorf("NextDueDate() = %v, must not equal due-based result %v", *got, notWant)
	}
}

// TestNextDueDate_FromCompletion_NilCompletedAt verifies the fallback to
// time.Now() when CompletedAt is nil in from-completion mode.
func TestNextDueDate_FromCompletion_NilCompletedAt(t *testing.T) {
	dueDate := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	task := &Task{
		DueDate:     &dueDate,
		CompletedAt: nil,
		Recurrence:  &RecurrencePattern{Interval: 3, Unit: RecurrenceDaily, Mode: RecurModeFromCompletion},
	}

	before := time.Now()
	got := task.NextDueDate()
	after := time.Now()

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	minExpected := before.AddDate(0, 0, 3)
	maxExpected := after.AddDate(0, 0, 3)
	if got.Before(minExpected) || got.After(maxExpected) {
		t.Errorf("NextDueDate() = %v, want between %v and %v", *got, minExpected, maxExpected)
	}
}

// TestGenerateRecurringInstances_ModeCarriesForward verifies that
// RecurModeFromCompletion persists across 2+ generated instances and that
// each generation's due date is computed from the prior instance's CompletedAt.
func TestGenerateRecurringInstances_ModeCarriesForward(t *testing.T) {
	dueDate := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2024, 3, 12, 10, 0, 0, 0, time.UTC) // 2 days late
	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring Task",
		Status:      StatusDone,
		DueDate:     &dueDate,
		CompletedAt: &completedAt,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Mode: RecurModeFromCompletion},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	// --- Generation 1 ---
	count := project.GenerateRecurringInstances()
	if count != 1 {
		t.Fatalf("generation 1: count = %d, want 1", count)
	}
	gen1 := project.Tasks[0]
	if gen1.Recurrence == nil {
		t.Fatal("generation 1: Recurrence is nil")
	}
	if gen1.Recurrence.Mode != RecurModeFromCompletion {
		t.Errorf("generation 1: Mode = %q, want %q", gen1.Recurrence.Mode, RecurModeFromCompletion)
	}
	wantDue1 := completedAt.AddDate(0, 0, 7) // completedAt + 1 week
	if gen1.DueDate == nil || !gen1.DueDate.Equal(wantDue1) {
		t.Errorf("generation 1: DueDate = %v, want %v", gen1.DueDate, wantDue1)
	}

	// --- Generation 2 ---
	gen1CompletedAt := time.Date(2024, 3, 20, 15, 0, 0, 0, time.UTC)
	gen1.Status = StatusDone
	gen1.CompletedAt = &gen1CompletedAt

	count = project.GenerateRecurringInstances()
	if count != 1 {
		t.Fatalf("generation 2: count = %d, want 1", count)
	}
	gen2 := project.Tasks[0]
	if gen2.Recurrence == nil {
		t.Fatal("generation 2: Recurrence is nil")
	}
	if gen2.Recurrence.Mode != RecurModeFromCompletion {
		t.Errorf("generation 2: Mode = %q, want %q", gen2.Recurrence.Mode, RecurModeFromCompletion)
	}
	wantDue2 := gen1CompletedAt.AddDate(0, 0, 7) // gen1CompletedAt + 1 week
	if gen2.DueDate == nil || !gen2.DueDate.Equal(wantDue2) {
		t.Errorf("generation 2: DueDate = %v, want %v", gen2.DueDate, wantDue2)
	}
}
