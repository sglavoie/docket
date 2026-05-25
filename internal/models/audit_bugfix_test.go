package models

import (
	"testing"
	"time"
)

// TestMonthlyRecurrenceNoDriftWhenSetRecurrenceBeforeDueDate verifies that
// monthly recurrence does not drift the day-of-month when SetRecurrence was
// called before DueDate was assigned (so OriginalDay was 0).
//
// Without the fix, the first short-month clamp would silently overwrite the
// user's intended day, causing the series to drift: Jan 31 → Feb 29 → Mar 29
// (wrong: should be Mar 31) → Apr 29 → ... permanently locked to the 29th.
func TestMonthlyRecurrenceNoDriftWhenSetRecurrenceBeforeDueDate(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Recurring monthly")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	// SetRecurrence BEFORE assigning DueDate (common flow when --recur is
	// passed to `tt task add` without a due date).
	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	if task.Recurrence.OriginalDay != 0 {
		t.Fatalf("OriginalDay = %d, want 0 (no DueDate yet)", task.Recurrence.OriginalDay)
	}

	// User now assigns Jan 31 as the due date.
	jan31 := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	task.DueDate = &jan31

	// First next-date computation: Feb 29 (leap year, clamped from 31).
	next := task.NextDueDate()
	if next == nil {
		t.Fatal("NextDueDate returned nil")
	}
	wantFeb := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	if !next.Equal(wantFeb) {
		t.Errorf("first next = %v, want %v", next, wantFeb)
	}

	// After advancing to Feb 29, the next should be Mar 31, NOT Mar 29.
	// The fix seeds OriginalDay from the original DueDate so the day is
	// restored once we land in a month that has 31 days.
	task.DueDate = next
	next2 := task.NextDueDate()
	if next2 == nil {
		t.Fatal("second NextDueDate returned nil")
	}
	wantMar := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	if !next2.Equal(wantMar) {
		t.Errorf("second next = %v, want %v (drift bug)", next2, wantMar)
	}

	// Continue: Mar 31 → Apr 30 (clamped), then May 31 (restored).
	task.DueDate = next2
	next3 := task.NextDueDate()
	wantApr := time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC)
	if !next3.Equal(wantApr) {
		t.Errorf("third next = %v, want %v", next3, wantApr)
	}

	task.DueDate = next3
	next4 := task.NextDueDate()
	wantMay := time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)
	if !next4.Equal(wantMay) {
		t.Errorf("fourth next = %v, want %v (drift bug)", next4, wantMay)
	}
}

// TestSkipRecurrenceSeedsOriginalDay verifies that SkipRecurrence also seeds
// OriginalDay so the same drift bug does not surface via the skip path.
func TestSkipRecurrenceSeedsOriginalDay(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Recurring monthly")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	jan31 := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	task.DueDate = &jan31

	task.SkipRecurrence()
	if task.Recurrence.OriginalDay != 31 {
		t.Errorf("OriginalDay = %d after Skip, want 31", task.Recurrence.OriginalDay)
	}
	wantFeb := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	if !task.DueDate.Equal(wantFeb) {
		t.Errorf("DueDate after Skip = %v, want %v", task.DueDate, wantFeb)
	}

	// Second skip lands on Mar 31, restored from OriginalDay.
	task.SkipRecurrence()
	wantMar := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	if !task.DueDate.Equal(wantMar) {
		t.Errorf("DueDate after second Skip = %v, want %v (drift)", task.DueDate, wantMar)
	}
}

// TestNextNDueDatesSeedsOriginalDay verifies that the preview iterator also
// seeds OriginalDay so the projected series does not drift.
func TestNextNDueDatesSeedsOriginalDay(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Recurring monthly preview")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	jan31 := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	task.DueDate = &jan31

	dates := task.NextNDueDates(4)
	if len(dates) != 4 {
		t.Fatalf("got %d dates, want 4", len(dates))
	}
	want := []time.Time{
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	for i, d := range dates {
		if !d.Equal(want[i]) {
			t.Errorf("dates[%d] = %v, want %v", i, d, want[i])
		}
	}
}
