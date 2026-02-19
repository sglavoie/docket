package models

import (
	"testing"
	"time"
)

func TestComplete(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionNext
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromWaiting(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionWaiting
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromSomeday(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionSomeday
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteKeepsCustomSection(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.CustomSection = "CustomSection"
	task.Complete()

	if task.CustomSection != "CustomSection" {
		t.Errorf("expected custom section preserved, got %q", task.CustomSection)
	}
}

func TestStart(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Start()

	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress, got %s", task.Status)
	}
}

func TestStartAutoTriagesFromInbox(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionInbox
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Inbox, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromNext(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionNext
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Next, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromReview(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionReview
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Review, got %q", task.Section)
	}
}

func TestBlock(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Block()

	if task.Status != StatusBlocked {
		t.Errorf("expected StatusBlocked, got %s", task.Status)
	}
}

func TestCancel(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Cancel()

	if task.Status != StatusCanceled {
		t.Errorf("expected StatusCanceled, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCancelMovesToCompletedSection(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionNext
	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestReview(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionStandby
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Standby, got %q", task.Section)
	}
}

func TestReopen(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
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
			task := NewTask("t1", 1, TaskTypeTask, "Test")
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
			task := NewTask("t1", 1, TaskTypeTask, "Test")
			task.ApplyTransition(tt.target)
			if task.Status != tt.wantStatus {
				t.Errorf("expected %s, got %s", tt.wantStatus, task.Status)
			}
		})
	}
}

func TestApplyTransitionUsesLifecycleMethods(t *testing.T) {
	// Verify ApplyTransition delegates to Start() which auto-triages
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	task.Section = SectionReview
	task.ApplyTransition(StatusInProgress)
	if task.Section != "" {
		t.Errorf("expected section cleared (Start auto-triage), got %q", task.Section)
	}

	// Verify ApplyTransition delegates to Complete() which sets CompletedAt
	task2 := NewTask("t2", 2, TaskTypeTask, "Test")
	task2.ApplyTransition(StatusDone)
	if task2.CompletedAt == nil {
		t.Error("expected CompletedAt set (Complete lifecycle), got nil")
	}
	if task2.Section != SectionCompleted {
		t.Errorf("expected Completed section, got %q", task2.Section)
	}

	// Verify ApplyTransition delegates to Reopen() which moves to Inbox
	task3 := NewTask("t3", 3, TaskTypeTask, "Test")
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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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
	task := NewTask("t1", 1, TaskTypeTask, "Test")

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

func TestIsRecurring(t *testing.T) {
	task := NewTask("t1", 1, TaskTypeTask, "Test")
	if task.IsRecurring() {
		t.Error("expected not recurring when nil")
	}

	task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1})
	if !task.IsRecurring() {
		t.Error("expected recurring after SetRecurrence")
	}

	task.ClearRecurrence()
	if task.IsRecurring() {
		t.Error("expected not recurring after ClearRecurrence")
	}
}
