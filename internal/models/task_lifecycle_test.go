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
