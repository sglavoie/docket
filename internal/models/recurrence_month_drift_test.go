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

// TestYearlyRecurrenceDateDrift verifies that OriginalDay preserves the
// intended day-of-month across non-leap years for yearly recurrence on Feb 29.
func TestYearlyRecurrenceDateDrift(t *testing.T) {
	r := &RecurrencePattern{
		Interval:    1,
		Unit:        RecurrenceYearly,
		OriginalDay: 29,
	}

	// Feb 29, 2024 (leap year) → Feb 28, 2025 (clamped, non-leap)
	feb29 := time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC)
	feb28 := calculateNextDate(feb29, r)
	if feb28.Day() != 28 || feb28.Month() != time.February || feb28.Year() != 2025 {
		t.Errorf("Feb 29 2024 + 1y = %v, want Feb 28 2025", feb28)
	}

	// Feb 28, 2025 → Feb 28, 2026 (clamped, non-leap; OriginalDay=29 but Feb has 28)
	feb28_2026 := calculateNextDate(feb28, r)
	if feb28_2026.Day() != 28 || feb28_2026.Month() != time.February || feb28_2026.Year() != 2026 {
		t.Errorf("Feb 28 2025 (orig 29) + 1y = %v, want Feb 28 2026", feb28_2026)
	}

	// Feb 28, 2026 → Feb 28, 2027 (clamped, non-leap)
	feb28_2027 := calculateNextDate(feb28_2026, r)
	if feb28_2027.Day() != 28 || feb28_2027.Month() != time.February || feb28_2027.Year() != 2027 {
		t.Errorf("Feb 28 2026 (orig 29) + 1y = %v, want Feb 28 2027", feb28_2027)
	}

	// Feb 28, 2027 → Feb 29, 2028 (restored via OriginalDay, leap year!)
	feb29_2028 := calculateNextDate(feb28_2027, r)
	if feb29_2028.Day() != 29 || feb29_2028.Month() != time.February || feb29_2028.Year() != 2028 {
		t.Errorf("Feb 28 2027 (orig 29) + 1y = %v, want Feb 29 2028", feb29_2028)
	}
}

// TestSetRecurrenceSetsOriginalDayForYearly verifies that SetRecurrence
// populates OriginalDay from the task's DueDate for yearly patterns.
func TestSetRecurrenceSetsOriginalDayForYearly(t *testing.T) {
	due := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)
	task := &Task{DueDate: &due, Tags: []string{}, DependsOn: []string{}, RelatedTo: []string{}}

	pattern := &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if task.Recurrence.OriginalDay != 29 {
		t.Errorf("OriginalDay = %d, want 29", task.Recurrence.OriginalDay)
	}
}

// TestSetRecurrenceNoOriginalDayForNonMonthlyYearly verifies that SetRecurrence
// does NOT set OriginalDay for non-monthly/yearly patterns.
func TestSetRecurrenceNoOriginalDayForNonMonthlyYearly(t *testing.T) {
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
