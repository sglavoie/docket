package models

import (
	"testing"
	"time"
)

func TestCompleteClearsDefer(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future

	task.Complete()

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Complete")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Complete")
	}
}

func TestCancelClearsDefer(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future

	task.Cancel()

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Cancel")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Cancel")
	}
}

func TestInboxTransitionClearsDeferAfterCompletion(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future
	task.Complete()
	if err := task.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Inbox transition")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Inbox transition")
	}
	if task.State() != StateInbox {
		t.Errorf("expected StateInbox after transition, got %s", task.State())
	}
}

func TestInboxTransitionClearsDeferDirectly(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 14)
	task.DeferredUntil = &future
	task.WorkflowState = StateDone
	now := time.Now()
	task.CompletedAt = &now

	if err := task.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Inbox transition")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Inbox transition")
	}
}

func TestStartClearsDefer(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future

	task.Start()

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Start")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Start")
	}
	if task.State() != StateDoing {
		t.Errorf("expected StateDoing after Start, got %s", task.State())
	}
}

func TestWaitClearsDefer(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDoing
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future

	task.Wait()

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Wait")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Wait")
	}
	if task.State() != StateWaiting {
		t.Errorf("expected StateWaiting after Wait, got %s", task.State())
	}
}

func TestStandbyClearsDefer(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDoing
	future := time.Now().AddDate(0, 0, 7)
	task.DeferredUntil = &future

	task.Standby()

	if task.DeferredUntil != nil {
		t.Error("expected DeferredUntil to be nil after Standby")
	}
	if task.IsDeferred() {
		t.Error("expected not deferred after Standby")
	}
	if task.State() != StatePaused {
		t.Errorf("expected StatePaused after Standby, got %s", task.State())
	}
}
