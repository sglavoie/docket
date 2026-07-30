package models

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
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

func TestIsSnoozedAtComparesCalendarDatesAcrossLocations(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	snoozeLocation := time.FixedZone("snooze", -8*60*60)
	referenceLocation := time.FixedZone("reference", 9*60*60)
	snooze := time.Date(2026, 3, 10, 0, 0, 0, 0, snoozeLocation)
	task.SnoozedUntil = &snooze

	sameCalendarDate := time.Date(2026, 3, 10, 0, 30, 0, 0, referenceLocation)
	if task.IsSnoozedAt(sameCalendarDate) {
		t.Fatal("same calendar date remained snoozed because locations encoded different instants")
	}

	previousCalendarDate := time.Date(2026, 3, 9, 23, 30, 0, 0, referenceLocation)
	if !task.IsSnoozedAt(previousCalendarDate) {
		t.Fatal("previous calendar date should remain snoozed regardless of location")
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

}

func TestSkipRecurrence(t *testing.T) {
	t.Run("advances from scheduled due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		task.Recurrence = &RecurrencePattern{
			Unit: RecurrenceWeekly, Interval: 1,
		}

		task.SkipRecurrence()

		expected := time.Date(2025, 6, 8, 0, 0, 0, 0, time.Local)
		if task.DueDate == nil || !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, task.DueDate)
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

	t.Run("no due date falls back to today", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		task.DueDate = nil
		task.Recurrence = &RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
		}

		task.SkipRecurrence()

		if task.DueDate == nil {
			t.Fatal("expected DueDate to be set after skip")
		}
		// Date-only recurrence advances to tomorrow at midnight.
		expected := util.StartOfDay(time.Now()).AddDate(0, 0, 1)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected midnight-aligned tomorrow (%v), got %v", expected, task.DueDate)
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
		}

		task.DeferRecurrence(5)

		if task.DueDate == nil {
			t.Fatal("expected due date to be set")
		}
		expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected due date %v, got %v", expected, *task.DueDate)
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

	t.Run("normalizes deferred due date to date-only", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		base := time.Date(2025, 6, 10, 15, 30, 0, 0, time.Local)
		task.DueDate = &base
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
