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
	if task.Status != StatusTodo {
		t.Errorf("expected StatusTodo after Reopen, got %s", task.Status)
	}
}

func TestReopenClearsSnoozeDirectly(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress after Start, got %s", task.Status)
	}
}

func TestBlockClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
