package models

import (
	"testing"
	"time"
)

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

	// NextNDueDates must honor RecurModeFromCompletion the same way
	// NextDueDate does: when in from-completion mode, the first upcoming
	// date is anchored on CompletedAt (or now if nil) — not on the stale
	// DueDate of the prior occurrence. Otherwise the preview ("Upcoming"
	// in the recur editor / task detail) would show dates in the past for
	// a task that was already completed late.
	t.Run("from completion mode with stale due date", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local) // far in the past
		task.DueDate = &stale
		if err := task.SetRecurrence(&RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 1,
			Mode:     RecurModeFromCompletion,
		}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		// The first upcoming date must be anchored on "now" (within a second
		// of NextDueDate), not on the stale 2020-era DueDate.
		nextDue := task.NextDueDate()
		if nextDue == nil {
			t.Fatal("NextDueDate() = nil")
		}
		delta := dates[0].Sub(*nextDue)
		if delta < -time.Second || delta > time.Second {
			t.Errorf("dates[0]=%v must be near NextDueDate=%v (delta=%v)", dates[0], *nextDue, delta)
		}
		// And dates must be in the future, not 2020-era.
		now := time.Now()
		if dates[0].Before(now.AddDate(-1, 0, 0)) {
			t.Errorf("dates[0]=%v is far in the past; from-completion mode must anchor on now", dates[0])
		}
	})

	t.Run("from completion mode uses CompletedAt when set", func(t *testing.T) {
		task := newLifecycleTestTask(t, "t1", 1)
		due := time.Date(2024, 3, 10, 0, 0, 0, 0, time.Local)
		completed := time.Date(2024, 3, 15, 9, 0, 0, 0, time.Local) // 5 days late
		task.DueDate = &due
		task.CompletedAt = &completed
		if err := task.SetRecurrence(&RecurrencePattern{
			Unit:     RecurrenceDaily,
			Interval: 3,
			Mode:     RecurModeFromCompletion,
		}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(2)
		if len(dates) != 2 {
			t.Fatalf("expected 2 dates, got %d", len(dates))
		}
		want0 := completed.AddDate(0, 0, 3)
		want1 := completed.AddDate(0, 0, 6)
		if !dates[0].Equal(want0) {
			t.Errorf("dates[0]=%v, want %v", dates[0], want0)
		}
		if !dates[1].Equal(want1) {
			t.Errorf("dates[1]=%v, want %v", dates[1], want1)
		}
	})
}
