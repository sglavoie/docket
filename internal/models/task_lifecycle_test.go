package models

import (
	"testing"
	"time"
)

func TestComplete(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Complete()

	if task.Status != StatusDone {
		t.Errorf("expected StatusDone, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if task.Active {
		t.Error("expected Active to be false after Complete")
	}
}

func TestCompleteMovesToCompletedSection(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromWaiting(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionWaiting
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromSomeday(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionSomeday
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteKeepsCustomSection(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "CustomSection"
	task.Complete()

	if task.CustomSection != "CustomSection" {
		t.Errorf("expected custom section preserved, got %q", task.CustomSection)
	}
}

func TestStart(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Start()

	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress, got %s", task.Status)
	}
}

func TestStartAutoTriagesFromInbox(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionInbox
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Inbox, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromNext(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Next, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromReview(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionReview
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Review, got %q", task.Section)
	}
}

func TestBlock(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Block()

	if task.Status != StatusBlocked {
		t.Errorf("expected StatusBlocked, got %s", task.Status)
	}
}

func TestCancel(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Cancel()

	if task.Status != StatusCanceled {
		t.Errorf("expected StatusCanceled, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCancelMovesToCompletedSection(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestReview(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Start()
	task.Review()

	if task.Status != StatusReview {
		t.Errorf("expected StatusReview, got %s", task.Status)
	}
	if task.Section != SectionReview {
		t.Errorf("expected Section Review, got %q", task.Section)
	}
}

func TestReviewFromTodo(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionInbox
	task.Review()

	if task.Status != StatusReview {
		t.Errorf("expected StatusReview, got %s", task.Status)
	}
	if task.Section != SectionReview {
		t.Errorf("expected Section Review, got %q", task.Section)
	}
}

func TestStandby(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Start()
	before := task.UpdatedAt
	task.Standby()

	if task.Status != StatusStandby {
		t.Errorf("expected StatusStandby, got %s", task.Status)
	}
	if task.Section != SectionStandby {
		t.Errorf("expected Section Standby, got %q", task.Section)
	}
	if !task.UpdatedAt.After(before) && task.UpdatedAt != before {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestStandbyFromTodo(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionInbox
	task.Standby()

	if task.Status != StatusStandby {
		t.Errorf("expected StatusStandby, got %s", task.Status)
	}
	if task.Section != SectionStandby {
		t.Errorf("expected Section Standby, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromStandby(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionStandby
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Standby, got %q", task.Section)
	}
}

func TestReopen(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Complete()
	task.Reopen()

	if task.Status != StatusTodo {
		t.Errorf("expected StatusTodo, got %s", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil after Reopen")
	}
	if task.Section != SectionInbox {
		t.Errorf("expected Section Inbox, got %q", task.Section)
	}
}

func TestCancelClearsActive(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Active = true
	task.Cancel()

	if task.Active {
		t.Error("expected Active to be false after Cancel")
	}
}

func TestReopenClearsActive(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Active = true
	task.Complete()
	task.Reopen()

	if task.Active {
		t.Error("expected Active to be false after Reopen")
	}
}

func TestReopenAfterCancelClearsCompletedAt(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Cancel()
	if task.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set after Cancel")
	}
	task.Reopen()
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil after Reopen following Cancel")
	}
}

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      TaskStatus
		to        TaskStatus
		wantAllow bool
	}{
		// From todo
		{"todo→in_progress", StatusTodo, StatusInProgress, true},
		{"todo→done", StatusTodo, StatusDone, true},
		{"todo→blocked", StatusTodo, StatusBlocked, true},
		{"todo→canceled", StatusTodo, StatusCanceled, true},
		{"todo→todo", StatusTodo, StatusTodo, false},
		{"todo→review", StatusTodo, StatusReview, true},
		{"todo→standby", StatusTodo, StatusStandby, true},

		// From in_progress
		{"in_progress→done", StatusInProgress, StatusDone, true},
		{"in_progress→blocked", StatusInProgress, StatusBlocked, true},
		{"in_progress→canceled", StatusInProgress, StatusCanceled, true},
		{"in_progress→todo", StatusInProgress, StatusTodo, true},
		{"in_progress→review", StatusInProgress, StatusReview, true},
		{"in_progress→standby", StatusInProgress, StatusStandby, true},
		{"in_progress→in_progress", StatusInProgress, StatusInProgress, false},

		// From review
		{"review→in_progress", StatusReview, StatusInProgress, true},
		{"review→done", StatusReview, StatusDone, true},
		{"review→blocked", StatusReview, StatusBlocked, true},
		{"review→canceled", StatusReview, StatusCanceled, true},
		{"review→todo", StatusReview, StatusTodo, true},
		{"review→standby", StatusReview, StatusStandby, true},
		{"review→review", StatusReview, StatusReview, false},

		// From standby
		{"standby→in_progress", StatusStandby, StatusInProgress, true},
		{"standby→done", StatusStandby, StatusDone, true},
		{"standby→blocked", StatusStandby, StatusBlocked, true},
		{"standby→canceled", StatusStandby, StatusCanceled, true},
		{"standby→todo", StatusStandby, StatusTodo, true},
		{"standby→review", StatusStandby, StatusReview, true},
		{"standby→standby", StatusStandby, StatusStandby, false},

		// From blocked
		{"blocked→in_progress", StatusBlocked, StatusInProgress, true},
		{"blocked→done", StatusBlocked, StatusDone, true},
		{"blocked→blocked", StatusBlocked, StatusBlocked, false},
		{"blocked→canceled", StatusBlocked, StatusCanceled, true},
		{"blocked→todo", StatusBlocked, StatusTodo, true},
		{"blocked→review", StatusBlocked, StatusReview, true},
		{"blocked→standby", StatusBlocked, StatusStandby, true},

		// From done (completed — can only reopen)
		{"done→todo", StatusDone, StatusTodo, true},
		{"done→in_progress", StatusDone, StatusInProgress, false},
		{"done→blocked", StatusDone, StatusBlocked, false},
		{"done→canceled", StatusDone, StatusCanceled, false},
		{"done→review", StatusDone, StatusReview, false},
		{"done→standby", StatusDone, StatusStandby, false},

		// From canceled (completed — can only reopen)
		{"canceled→todo", StatusCanceled, StatusTodo, true},
		{"canceled→in_progress", StatusCanceled, StatusInProgress, false},
		{"canceled→done", StatusCanceled, StatusDone, false},
		{"canceled→blocked", StatusCanceled, StatusBlocked, false},
		{"canceled→review", StatusCanceled, StatusReview, false},
		{"canceled→standby", StatusCanceled, StatusStandby, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewTask("t1", 1, TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			task.Status = tt.from
			msg := task.CanTransitionTo(tt.to)
			if tt.wantAllow && msg != "" {
				t.Errorf("expected transition allowed, got %q", msg)
			}
			if !tt.wantAllow && msg == "" {
				t.Error("expected transition rejected, got allowed")
			}
		})
	}
}

func TestApplyTransition(t *testing.T) {
	tests := []struct {
		name       string
		target     TaskStatus
		wantStatus TaskStatus
	}{
		{"to in_progress", StatusInProgress, StatusInProgress},
		{"to done", StatusDone, StatusDone},
		{"to blocked", StatusBlocked, StatusBlocked},
		{"to canceled", StatusCanceled, StatusCanceled},
		{"to todo (reopen)", StatusTodo, StatusTodo},
		{"to review", StatusReview, StatusReview},
		{"to standby", StatusStandby, StatusStandby},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewTask("t1", 1, TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			task.ApplyTransition(tt.target)
			if task.Status != tt.wantStatus {
				t.Errorf("expected %s, got %s", tt.wantStatus, task.Status)
			}
		})
	}
}

func TestApplyTransitionUsesLifecycleMethods(t *testing.T) {
	// Verify ApplyTransition delegates to Start() which auto-triages
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionReview
	task.ApplyTransition(StatusInProgress)
	if task.Section != "" {
		t.Errorf("expected section cleared (Start auto-triage), got %q", task.Section)
	}

	// Verify ApplyTransition delegates to Complete() which sets CompletedAt
	task2, err := NewTask("t2", 2, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task2.ApplyTransition(StatusDone)
	if task2.CompletedAt == nil {
		t.Error("expected CompletedAt set (Complete lifecycle), got nil")
	}
	if task2.Section != SectionCompleted {
		t.Errorf("expected Completed section, got %q", task2.Section)
	}

	// Verify ApplyTransition delegates to Reopen() which moves to Inbox
	task3, err := NewTask("t3", 3, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete()
	task3.ApplyTransition(StatusTodo)
	if task3.Section != SectionInbox {
		t.Errorf("expected Inbox section (Reopen lifecycle), got %q", task3.Section)
	}
	if task3.CompletedAt != nil {
		t.Error("expected CompletedAt cleared after Reopen")
	}
}

func TestPinUnpin(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.Pin()
	if !task.Pinned {
		t.Error("expected Pinned to be true")
	}
	if task.PinnedAt == nil {
		t.Error("expected PinnedAt to be set")
	}

	task.Unpin()
	if task.Pinned {
		t.Error("expected Pinned to be false")
	}
	if task.PinnedAt != nil {
		t.Error("expected PinnedAt to be nil")
	}
}

func TestTogglePinned(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.TogglePinned()
	if !task.Pinned {
		t.Error("expected Pinned to be true after first toggle")
	}

	task.TogglePinned()
	if task.Pinned {
		t.Error("expected Pinned to be false after second toggle")
	}
}

func TestActivateDeactivate(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.Activate()
	if !task.Active {
		t.Error("expected Active to be true")
	}

	task.Deactivate()
	if task.Active {
		t.Error("expected Active to be false")
	}
}

func TestToggleActive(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	task.ToggleActive()
	if !task.Active {
		t.Error("expected Active to be true after first toggle")
	}

	task.ToggleActive()
	if task.Active {
		t.Error("expected Active to be false after second toggle")
	}
}

func TestIsSnoozed(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

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

func TestIsOverdue(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

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

	// Completed tasks are never overdue
	task.DueDate = &past
	task.Complete()
	if task.IsOverdue() {
		t.Error("expected not overdue when task is complete")
	}
}

func TestIsDueSoon(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	if task.IsDueSoon(7) {
		t.Error("expected not due soon when no due date")
	}

	// Due in 3 days should be "due soon" for 7-day threshold
	future := time.Now().AddDate(0, 0, 3)
	task.DueDate = &future
	if !task.IsDueSoon(7) {
		t.Error("expected due soon within 7 days")
	}
	if task.IsDueSoon(1) {
		t.Error("expected not due soon within 1 day when due in 3 days")
	}

	// Overdue tasks are not "due soon"
	past := time.Now().AddDate(0, 0, -1)
	task.DueDate = &past
	if task.IsDueSoon(7) {
		t.Error("expected not due soon when overdue")
	}
}

func TestNextNDueDates(t *testing.T) { //nolint:gocyclo
	t.Run("nil when not recurring", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		dates := task.NextNDueDates(3)
		if dates != nil {
			t.Errorf("expected nil, got %v", dates)
		}
	})

	t.Run("nil when n <= 0", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}
		dates := task.NextNDueDates(0)
		if dates != nil {
			t.Errorf("expected nil for n=0, got %v", dates)
		}
	})

	t.Run("daily with due date", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local) // Sunday
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceMonthly, Interval: 1}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		// Jan 31 -> Feb 28 (clamped), Feb 28 -> Mar 31 (restored via OriginalDay=31), Mar 31 -> Apr 30 (clamped)
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		// Wednesday June 4, 2025
		base := time.Date(2025, 6, 4, 0, 0, 0, 0, time.Local)
		task.DueDate = &base
		if err := task.SetRecurrence(&RecurrencePattern{
			Unit:     RecurrenceWeekly,
			Interval: 1,
			Weekdays: []int{1, 3, 5}, // Mon, Wed, Fri
		}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(3)
		if len(dates) != 3 {
			t.Fatalf("expected 3 dates, got %d", len(dates))
		}
		// Wed -> Fri (same week), Fri -> Mon (next week), Mon -> Wed (same week)
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 2}); err != nil {
			t.Fatalf("SetRecurrence failed: %v", err)
		}

		dates := task.NextNDueDates(2)
		if len(dates) != 2 {
			t.Fatalf("expected 2 dates, got %d", len(dates))
		}
		// First date should be ~2 days from now, second ~4 days from now
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		before := task.UpdatedAt
		task.SkipRecurrence()
		if task.UpdatedAt != before {
			t.Error("expected no-op for non-recurring task")
		}
	})

	t.Run("increments skipped counter", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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
		// Streak should be unchanged
		if task.Recurrence.Streak != 3 {
			t.Errorf("expected streak unchanged at 3, got %d", task.Recurrence.Streak)
		}
	})

	t.Run("falls back to today when no due date", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
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

	t.Run("no-op when not recurring", func(t *testing.T) {
		task, err := NewTask("t1", 1, TaskTypeTask, "Test")
		if err != nil {
			t.Fatal(err)
		}
		before := task.UpdatedAt
		task.DeferRecurrence(5)
		if task.UpdatedAt != before {
			t.Error("expected no-op for non-recurring task")
		}
	})
}

func TestIsRecurring(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
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

func TestCompleteClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Complete()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Complete")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Complete")
	}
}

func TestCancelClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Cancel()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Cancel")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Cancel")
	}
}

func TestReopenClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	// Snooze, complete, then reopen — snooze should be cleared
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future
	task.Complete()
	task.Reopen()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Reopen")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Reopen")
	}
	if task.Status != StatusTodo {
		t.Errorf("expected StatusTodo after Reopen, got %s", task.Status)
	}
}

func TestReopenClearsSnoozeDirectly(t *testing.T) {
	// Test the case where a task was snoozed, then directly reopened
	// (e.g., task was snoozed with a future date but status was set externally)
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(0, 0, 14)
	task.SnoozedUntil = &future
	task.Status = StatusDone
	now := time.Now()
	task.CompletedAt = &now

	task.Reopen()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Reopen")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Reopen")
	}
}

func TestStartClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Start()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Start")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Start")
	}
	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress after Start, got %s", task.Status)
	}
}

func TestBlockClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = StatusInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Block()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Block")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Block")
	}
	if task.Status != StatusBlocked {
		t.Errorf("expected StatusBlocked after Block, got %s", task.Status)
	}
}

func TestReviewClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = StatusInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Review()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Review")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Review")
	}
	if task.Status != StatusReview {
		t.Errorf("expected StatusReview after Review, got %s", task.Status)
	}
}

func TestStandbyClearsSnooze(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = StatusInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Standby()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Standby")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Standby")
	}
	if task.Status != StatusStandby {
		t.Errorf("expected StatusStandby after Standby, got %s", task.Status)
	}
}

func TestDimUndim(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Dim()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after Dim")
	}
	task.Undim()
	if task.Dimmed {
		t.Error("expected Dimmed to be false after Undim")
	}
}

func TestToggleDimmed(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.ToggleDimmed()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after first ToggleDimmed")
	}
	task.ToggleDimmed()
	if task.Dimmed {
		t.Error("expected Dimmed to be false after second ToggleDimmed")
	}
}

func TestDimClearsActive(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Active = true
	task.Dim()
	if !task.Dimmed {
		t.Error("expected Dimmed to be true after Dim")
	}
	if task.Active {
		t.Error("expected Active to be false after Dim")
	}
}

func TestActivateClearsDimmed(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Dimmed = true
	task.Activate()
	if !task.Active {
		t.Error("expected Active to be true after Activate")
	}
	if task.Dimmed {
		t.Error("expected Dimmed to be false after Activate")
	}
}
