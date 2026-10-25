package models

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/util"
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

// calendarDayIsDeferredAt is the predicate schema 9 shipped, kept verbatim so the
// instant comparison that replaced it can be measured against it.
func calendarDayIsDeferredAt(deferredUntil, now time.Time) bool {
	nowYear, nowMonth, nowDay := now.Date()
	deferYear, deferMonth, deferDayOfMonth := deferredUntil.Date()
	today := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)
	deferDay := time.Date(deferYear, deferMonth, deferDayOfMonth, 0, 0, 0, 0, time.UTC)
	return deferDay.After(today)
}

// TestIsDeferredAtMatchesTheCalendarDayPredicateItReplaced is the one behavioral
// equivalence the schema-10 bump rests on. Every deferral schema 9 could store is
// the instant some day began in the reader's own location, and the migration
// lifts each one there, so for those values the instant comparison must answer
// exactly what the calendar-day comparison answered. The weeks below each span a
// DST transition, including a zone whose transition lands on midnight itself.
func TestIsDeferredAtMatchesTheCalendarDayPredicateItReplaced(t *testing.T) {
	weeks := []struct {
		zone  string
		year  int
		month time.Month
		day   int
	}{
		{"America/Los_Angeles", 2026, time.March, 5},    // spring forward at 02:00
		{"America/Los_Angeles", 2026, time.October, 29}, // fall back at 02:00
		{"Asia/Beirut", 2026, time.March, 26},           // spring forward at 00:00
		{"Australia/Lord_Howe", 2026, time.April, 2},    // 30-minute transition
		{"UTC", 2026, time.March, 5},
	}
	hours := []int{0, 1, 2, 3, 12, 23}
	task := newLifecycleTestTask(t, "t1", 1)

	for _, week := range weeks {
		location, err := time.LoadLocation(week.zone)
		if err != nil {
			t.Fatalf("load %s: %v", week.zone, err)
		}
		for deferOffset := range 7 {
			// A schema-9 deferral always denoted the start of a day in the location
			// the bare date was read in, which is what the v11 lift writes down.
			deferredUntil := time.Date(week.year, week.month, week.day+deferOffset, 0, 0, 0, 0, location)
			task.DeferredUntil = &deferredUntil
			for nowOffset := range 7 {
				for _, hour := range hours {
					now := time.Date(week.year, week.month, week.day+nowOffset, hour, 30, 0, 0, location)
					want := calendarDayIsDeferredAt(deferredUntil, now)
					got := task.IsDeferredAt(now)
					if got != want {
						t.Fatalf("%s: IsDeferredAt(%s) with deferral %s = %v, want %v",
							week.zone, now.Format(time.RFC3339), deferredUntil.Format(time.RFC3339), got, want)
					}
					// The same instant read in another location is still the same
					// instant, which is what makes the comparison portable.
					if elsewhere := task.IsDeferredAt(now.In(time.UTC)); elsewhere != got {
						t.Fatalf("%s: IsDeferredAt(%s) changed to %v when the reference time moved location",
							week.zone, now.Format(time.RFC3339), elsewhere)
					}
				}
			}
		}
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

func TestIsOverdueAtDistinguishesDateOnlyAndTimedDeadlines(t *testing.T) {
	now := time.Date(2026, time.October, 16, 10, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name  string
		due   time.Time
		state WorkflowState
		want  bool
	}{
		{"earlier today", now.Add(-time.Hour), StateBacklog, true},
		{"exactly now", now, StateBacklog, false},
		{"later today", now.Add(time.Hour), StateBacklog, false},
		{"date only today", util.StartOfDay(now), StateBacklog, false},
		{"date only yesterday", util.StartOfDay(now).AddDate(0, 0, -1), StateBacklog, true},
		{"completed", now.Add(-time.Hour), StateDone, false},
		{"canceled", now.Add(-time.Hour), StateCanceled, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{WorkflowState: tc.state, DueDate: &tc.due}
			if got := task.IsOverdueAt(now); got != tc.want {
				t.Fatalf("IsOverdueAt = %v, want %v", got, tc.want)
			}
		})
	}
}
