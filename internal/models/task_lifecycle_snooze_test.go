package models

import (
	"testing"
	"time"
)

func TestCompleteClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
	if task.State() != StateInbox {
		t.Errorf("expected StateInbox after Reopen, got %s", task.State())
	}
}

func TestReopenClearsSnoozeDirectly(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 14)
	task.SnoozedUntil = &future
	task.WorkflowState = StateDone
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
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Start()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Start")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Start")
	}
	if task.State() != StateInProgress {
		t.Errorf("expected StateInProgress after Start, got %s", task.State())
	}
}

func TestBlockClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Block()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Block")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Block")
	}
	if task.State() != StateWaiting {
		t.Errorf("expected StateWaiting after Block, got %s", task.State())
	}
}

func TestReviewClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Review()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Review")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Review")
	}
	if task.State() != StateReview {
		t.Errorf("expected StateReview after Review, got %s", task.State())
	}
}

func TestStandbyClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInProgress
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Standby()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Standby")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Standby")
	}
	if task.State() != StatePaused {
		t.Errorf("expected StatePaused after Standby, got %s", task.State())
	}
}
