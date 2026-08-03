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

func TestInboxTransitionClearsSnoozeAfterCompletion(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future
	task.Complete()
	if err := task.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Inbox transition")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Inbox transition")
	}
	if task.State() != StateInbox {
		t.Errorf("expected StateInbox after transition, got %s", task.State())
	}
}

func TestInboxTransitionClearsSnoozeDirectly(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 14)
	task.SnoozedUntil = &future
	task.WorkflowState = StateDone
	now := time.Now()
	task.CompletedAt = &now

	if err := task.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Inbox transition")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Inbox transition")
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
	if task.State() != StateDoing {
		t.Errorf("expected StateDoing after Start, got %s", task.State())
	}
}

func TestWaitClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDoing
	future := time.Now().AddDate(0, 0, 7)
	task.SnoozedUntil = &future

	task.Wait()

	if task.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be nil after Wait")
	}
	if task.IsSnoozed() {
		t.Error("expected not snoozed after Wait")
	}
	if task.State() != StateWaiting {
		t.Errorf("expected StateWaiting after Wait, got %s", task.State())
	}
}

func TestStandbyClearsSnooze(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDoing
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
