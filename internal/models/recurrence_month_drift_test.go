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

// TestNewRecurrencePatternSetsOriginalDay verifies that NewRecurrencePattern
// correctly records the due date's day for monthly recurrences.
func TestNewRecurrencePatternSetsOriginalDay(t *testing.T) {
	base := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	due := time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC)

	got := NewRecurrencePattern(base, due)
	if got.OriginalDay != 31 {
		t.Errorf("OriginalDay = %d, want 31", got.OriginalDay)
	}

	// Non-monthly: OriginalDay should not be set.
	baseDaily := &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}
	gotDaily := NewRecurrencePattern(baseDaily, due)
	if gotDaily.OriginalDay != 0 {
		t.Errorf("OriginalDay for daily = %d, want 0", gotDaily.OriginalDay)
	}

	// Already has OriginalDay set: should not overwrite.
	baseExisting := &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly, OriginalDay: 15}
	gotExisting := NewRecurrencePattern(baseExisting, due)
	if gotExisting.OriginalDay != 15 {
		t.Errorf("OriginalDay should not be overwritten: got %d, want 15", gotExisting.OriginalDay)
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
