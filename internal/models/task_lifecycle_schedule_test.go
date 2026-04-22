package models

import (
	"testing"
	"time"
)

func TestIsSnoozed(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	if task.IsSnoozed() {
		t.Error("expected not snoozed when SnoozedUntil is nil")
	}

	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future
	if !task.IsSnoozed() {
		t.Error("expected snoozed when SnoozedUntil is in the future")
	}

	past := time.Now().AddDate(0, 0, -1)
	task.SnoozedUntil = &past
	if task.IsSnoozed() {
		t.Error("expected not snoozed when SnoozedUntil is in the past")
	}
}

func TestIsSnoozedAt(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	// nil SnoozedUntil => not snoozed at any time
	refTime := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	if task.IsSnoozedAt(refTime) {
		t.Error("expected not snoozed when SnoozedUntil is nil")
	}

	// Snoozed until March 10; reference time is Feb 28 => snoozed
	snooze := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	task.SnoozedUntil = &snooze
	ref := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	if !task.IsSnoozedAt(ref) {
		t.Error("expected snoozed when reference time is before snooze date")
	}

	// Same snooze date; reference time is March 15 => not snoozed
	ref = time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	if task.IsSnoozedAt(ref) {
		t.Error("expected not snoozed when reference time is after snooze date")
	}

	// Snooze day equals reference day => not snoozed (snooze expires on that day)
	ref = time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	if task.IsSnoozedAt(ref) {
		t.Error("expected not snoozed when reference time equals snooze date")
	}
}

func TestIsOverdue(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	if task.IsOverdue() {
		t.Error("expected not overdue when no due date")
	}

	past := time.Now().AddDate(0, 0, -2)
	task.DueDate = &past
	if !task.IsOverdue() {
		t.Error("expected overdue when due date is in the past")
	}

	future := time.Now().AddDate(0, 0, 2)
	task.DueDate = &future
	if task.IsOverdue() {
		t.Error("expected not overdue when due date is in the future")
	}

	task.DueDate = &past
	task.Complete()
	if task.IsOverdue() {
		t.Error("expected not overdue when task is complete")
	}
}

func TestIsDueSoon(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	if task.IsDueSoon(7) {
		t.Error("expected not due soon when no due date")
	}

	future := time.Now().AddDate(0, 0, 3)
	task.DueDate = &future
	if !task.IsDueSoon(7) {
		t.Error("expected due soon within 7 days")
	}
	if task.IsDueSoon(1) {
		t.Error("expected not due soon within 1 day when due in 3 days")
	}

	past := time.Now().AddDate(0, 0, -1)
	task.DueDate = &past
	if task.IsDueSoon(7) {
		t.Error("expected not due soon when overdue")
	}

	// HasTime: task due earlier today should be overdue, not "due soon"
	pastTime := time.Now().Add(-2 * time.Hour)
	task.DueDate = &pastTime
	task.HasTime = true
	if task.IsDueSoon(7) {
		t.Error("expected not due soon when HasTime and due time has passed")
	}

	// HasTime: task due later today should be "due soon"
	futureTime := time.Now().Add(2 * time.Hour)
	task.DueDate = &futureTime
	task.HasTime = true
	if !task.IsDueSoon(7) {
		t.Error("expected due soon when HasTime and due time is within 7 days")
	}
}

func TestNextNDueDates(t *testing.T) { //nolint:gocyclo
	t.Run("nil when not recurring", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		dates := task.NextNDueDates(3)
		if dates != nil {
			t.Errorf("expected nil, got %v", dates)
		}
	})

	t.Run("nil when n <= 0", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}
		dates := task.NextNDueDates(0)
		if dates != nil {
			t.Errorf("expected nil for n=0, got %v", dates)
		}
	})

	t.Run("daily with due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		expected := []time.Time{
			time.Date(2025, 6, 2, 0, 0, 0, 0, time.Local),
			time.Date(2025, 6, 3, 0, 0, 0, 0, time.Local),
			time.Date(2025, 6, 4, 0, 0, 0, 0, time.Local),
		}
		for i, d := range dates {
			if !d.Equal(expected[i]) {
				t.Errorf("date[%d]: expected %v, got %v", i, expected[i], d)
			}
		}
	})

	t.Run("weekly with due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceWeekly, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		expected := []time.Time{
			time.Date(2025, 6, 8, 0, 0, 0, 0, time.Local),
			time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local),
			time.Date(2025, 6, 22, 0, 0, 0, 0, time.Local),
		}
		for i, d := range dates {
			if !d.Equal(expected[i]) {
				t.Errorf("date[%d]: expected %v, got %v", i, expected[i], d)
			}
		}
	})

	t.Run("monthly with due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceMonthly, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		if dates[0].Month() != time.February || dates[0].Day() != 28 {
			t.Errorf("date[0]: expected Feb 28, got %v", dates[0])
		}
		if dates[1].Month() != time.March || dates[1].Day() != 31 {
			t.Errorf("date[1]: expected Mar 31, got %v", dates[1])
		}
		if dates[2].Month() != time.April || dates[2].Day() != 30 {
			t.Errorf("date[2]: expected Apr 30, got %v", dates[2])
		}
	})

	t.Run("yearly with due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceYearly, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		expected := []time.Time{
			time.Date(2026, 3, 15, 0, 0, 0, 0, time.Local),
			time.Date(2027, 3, 15, 0, 0, 0, 0, time.Local),
			time.Date(2028, 3, 15, 0, 0, 0, 0, time.Local),
		}
		for i, d := range dates {
			if !d.Equal(expected[i]) {
				t.Errorf("date[%d]: expected %v, got %v", i, expected[i], d)
			}
		}
	})

	t.Run("weekly with specific weekdays", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 4, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{
			Unit:     RecurrenceWeekly,
			Interval: 1,
			Weekdays: []int{1, 3, 5},
		}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		if dates[0].Weekday() != time.Friday {
			t.Errorf("date[0]: expected Friday, got %v", dates[0].Weekday())
		}
		if dates[1].Weekday() != time.Monday {
			t.Errorf("date[1]: expected Monday, got %v", dates[1].Weekday())
		}
		if dates[2].Weekday() != time.Wednesday {
			t.Errorf("date[2]: expected Wednesday, got %v", dates[2].Weekday())
		}
	})

	t.Run("no due date uses today as base", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 2}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(2)
		if len(dates) != 2 {
			t.Fatalf("expected 2 dates, got %d", len(dates))
		}
		now := time.Now()
		diff1 := dates[0].Sub(now).Hours() / 24
		if diff1 < 1.5 || diff1 > 2.5 {
			t.Errorf("date[0]: expected ~2 days from now, got %.1f days", diff1)
		}
		diff2 := dates[1].Sub(now).Hours() / 24
		if diff2 < 3.5 || diff2 > 4.5 {
			t.Errorf("date[1]: expected ~4 days from now, got %.1f days", diff2)
		}
	})

	t.Run("every 2 weeks", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceWeekly, Interval: 2}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		expected := []time.Time{
			time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local),
			time.Date(2025, 6, 29, 0, 0, 0, 0, time.Local),
			time.Date(2025, 7, 13, 0, 0, 0, 0, time.Local),
		}
		for i, d := range dates {
			if !d.Equal(expected[i]) {
				t.Errorf("date[%d]: expected %v, got %v", i, expected[i], d)
			}
		}
	})
}

func TestSkipRecurrence(t *testing.T) {
	t.Run("advances due date and resets streak", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceWeekly,
			Interval: 1,
			Streak:   5,
		}

		task.SkipRecurrence()

		expected := time.Date(2025, 6, 8, 0, 0, 0, 0, time.Local)
		if task.DueDate == nil || !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, task.DueDate)
		}
		if task.Recurrence.Streak != 0 {
			t.Errorf("expected streak 0 after skip, got %d", task.Recurrence.Streak)
		}
		if task.Recurrence.Skipped != 1 {
			t.Errorf("expected skipped 1, got %d", task.Recurrence.Skipped)
		}
	})

	t.Run("no-op when not recurring", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		before := task.UpdatedAt
		task.SkipRecurrence()
		if task.UpdatedAt != before {
			t.Error("expected no-op for non-recurring task")
		}
	})

	t.Run("increments skipped counter", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
			Skipped:  2,
		}

		task.SkipRecurrence()
		if task.Recurrence.Skipped != 3 {
			t.Errorf("expected skipped 3, got %d", task.Recurrence.Skipped)
		}
	})
}

func TestDeferRecurrence(t *testing.T) {
	t.Run("adds days to existing due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 10, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
			Streak:   3,
		}

		task.DeferRecurrence(5)

		if task.DueDate == nil {
			t.Fatal("expected due date to be set")
		}
		expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, *task.DueDate)
		}
		if task.Recurrence.Streak != 3 {
			t.Errorf("expected streak unchanged at 3, got %d", task.Recurrence.Streak)
		}
	})

	t.Run("falls back to today when no due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceWeekly,
			Interval: 1,
		}

		task.DeferRecurrence(3)

		if task.DueDate == nil {
			t.Fatal("expected due date to be set")
		}
		now := time.Now()
		expected := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, 3)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, *task.DueDate)
		}
	})

	t.Run("preserves time-of-day when HasTime is true", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 10, 15, 30, 0, 0, time.Local)
		task.DueDate = &base
		task.HasTime = true
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
		}

		task.DeferRecurrence(2)

		if task.DueDate == nil {
			t.Fatal("expected due date to be set")
		}
		expected := time.Date(2025, 6, 12, 15, 30, 0, 0, time.Local)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, *task.DueDate)
		}
	})

	t.Run("strips time when HasTime is false", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 10, 15, 30, 0, 0, time.Local)
		task.DueDate = &base
		task.HasTime = false
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
		}

		task.DeferRecurrence(2)

		if task.DueDate == nil {
			t.Fatal("expected due date to be set")
		}
		expected := time.Date(2025, 6, 12, 0, 0, 0, 0, time.Local)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, *task.DueDate)
		}
	})

	t.Run("no-op when not recurring", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		before := task.UpdatedAt
		task.DeferRecurrence(5)
		if task.UpdatedAt != before {
			t.Error("expected no-op for non-recurring task")
		}
	})
}

func TestIsRecurring(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	if task.IsRecurring() {
		t.Error("expected not recurring when nil")
	}

	if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1}); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	if !task.IsRecurring() {
		t.Error("expected recurring after SetRecurrence")
	}

	task.ClearRecurrence()
	if task.IsRecurring() {
		t.Error("expected not recurring after ClearRecurrence")
	}
}
