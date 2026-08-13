package models

import (
	"testing"
	"time"
)

func TestIsDeferred(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	if task.IsDeferred() {
		t.Error("expected not deferred when DeferredUntil is nil")
	}

	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future
	if !task.IsDeferred() {
		t.Error("expected deferred when DeferredUntil is in the future")
	}

	past := time.Now().AddDate(0, 0, -1)
	task.DeferredUntil = &past
	if task.IsDeferred() {
		t.Error("expected not deferred when DeferredUntil is in the past")
	}
}

func TestIsDeferredAt(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)

	// nil DeferredUntil => not deferred at any time
	refTime := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	if task.IsDeferredAt(refTime) {
		t.Error("expected not deferred when DeferredUntil is nil")
	}

	// Deferred until March 10; reference time is Feb 28 => deferred
	deferUntil := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	task.DeferredUntil = &deferUntil
	ref := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	if !task.IsDeferredAt(ref) {
		t.Error("expected deferred when reference time is before deferral date")
	}

	// Same deferral date; reference time is March 15 => not deferred
	ref = time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	if task.IsDeferredAt(ref) {
		t.Error("expected not deferred when reference time is after deferral date")
	}

	// Deferral day equals reference day => not deferred (deferral expires on that day)
	ref = time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	if task.IsDeferredAt(ref) {
		t.Error("expected not deferred when reference time equals deferral date")
	}
}

func TestIsDeferredAtComparesCalendarDatesAcrossLocations(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	deferLocation := time.FixedZone("defer", -8*60*60)
	referenceLocation := time.FixedZone("reference", 9*60*60)
	deferUntil := time.Date(2026, 3, 10, 0, 0, 0, 0, deferLocation)
	task.DeferredUntil = &deferUntil

	sameCalendarDate := time.Date(2026, 3, 10, 0, 30, 0, 0, referenceLocation)
	if task.IsDeferredAt(sameCalendarDate) {
		t.Fatal("same calendar date remained deferred because locations encoded different instants")
	}

	previousCalendarDate := time.Date(2026, 3, 9, 23, 30, 0, 0, referenceLocation)
	if !task.IsDeferredAt(previousCalendarDate) {
		t.Fatal("previous calendar date should remain deferred regardless of location")
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
