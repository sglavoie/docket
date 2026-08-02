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
