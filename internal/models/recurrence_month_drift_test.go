package models

import (
	"testing"
	"time"
)

// TestMonthlyRecurrenceDateDrift verifies that OriginalDay preserves the
// intended day-of-month across months with fewer days (e.g. Jan 31 → Mar 31).
func TestMonthlyRecurrenceDateDrift(t *testing.T) {
	// Start: Jan 31, monthly recurrence with OriginalDay=31.
	r := &RecurrencePattern{
		Interval:    1,
		Unit:        RecurrenceMonthly,
		OriginalDay: 31,
	}

	// Jan 31 → Feb 28 (clamped, non-leap 2023)
	jan31 := time.Date(2023, time.January, 31, 10, 0, 0, 0, time.UTC)
	feb28 := calculateNextDate(jan31, r)
	if feb28.Day() != 28 || feb28.Month() != time.February {
		t.Errorf("Jan 31 + 1m = %v, want Feb 28", feb28)
	}

	// Feb 28 → Mar 31 (restored via OriginalDay)
	mar31 := calculateNextDate(feb28, r)
	if mar31.Day() != 31 || mar31.Month() != time.March {
		t.Errorf("Feb 28 (orig 31) + 1m = %v, want Mar 31", mar31)
	}

	// Mar 31 → Apr 30 (clamped)
	apr30 := calculateNextDate(mar31, r)
	if apr30.Day() != 30 || apr30.Month() != time.April {
		t.Errorf("Mar 31 + 1m = %v, want Apr 30", apr30)
	}

	// Apr 30 → May 31 (restored via OriginalDay)
	may31 := calculateNextDate(apr30, r)
	if may31.Day() != 31 || may31.Month() != time.May {
		t.Errorf("Apr 30 (orig 31) + 1m = %v, want May 31", may31)
	}
}

// TestSetRecurrenceSetsOriginalDayFromDueDate verifies that SetRecurrence
// populates OriginalDay from the task's DueDate for monthly patterns.
func TestSetRecurrenceSetsOriginalDayFromDueDate(t *testing.T) {
	due := time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC)
	task := &Task{DueDate: &due, Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{}}

	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if task.Recurrence.OriginalDay != 31 {
		t.Errorf("OriginalDay = %d, want 31", task.Recurrence.OriginalDay)
	}
}

// TestSetRecurrenceNoOriginalDayForNonMonthly verifies that SetRecurrence
// does NOT set OriginalDay for non-monthly patterns.
func TestSetRecurrenceNoOriginalDayForNonMonthly(t *testing.T) {
	due := time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC)
	task := &Task{DueDate: &due, Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{}}

	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if task.Recurrence.OriginalDay != 0 {
		t.Errorf("OriginalDay = %d, want 0 for weekly", task.Recurrence.OriginalDay)
	}
}

// TestSetRecurrencePreservesExistingOriginalDay verifies that SetRecurrence
// does NOT overwrite a pre-existing OriginalDay.
func TestSetRecurrencePreservesExistingOriginalDay(t *testing.T) {
	due := time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC)
	task := &Task{DueDate: &due, Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{}}

	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly, OriginalDay: 15}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if task.Recurrence.OriginalDay != 15 {
		t.Errorf("OriginalDay = %d, want 15 (should not be overwritten)", task.Recurrence.OriginalDay)
	}
}

// TestSetRecurrenceNoDueDateNoOriginalDay verifies that SetRecurrence
// does NOT set OriginalDay when the task has no DueDate.
func TestSetRecurrenceNoDueDateNoOriginalDay(t *testing.T) {
	task := &Task{Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{}}

	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if task.Recurrence.OriginalDay != 0 {
		t.Errorf("OriginalDay = %d, want 0 (no due date set)", task.Recurrence.OriginalDay)
	}
}

// TestMonthlyRecurrenceBackwardCompat verifies that OriginalDay==0 (old data)
// falls back to using base.Day() unchanged.
func TestMonthlyRecurrenceBackwardCompat(t *testing.T) {
	r := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly} // OriginalDay == 0

	jan15 := time.Date(2023, time.January, 15, 10, 0, 0, 0, time.UTC)
	feb15 := calculateNextDate(jan15, r)
	if feb15.Day() != 15 || feb15.Month() != time.February {
		t.Errorf("Jan 15 + 1m = %v, want Feb 15", feb15)
	}
}
