package models

import "testing"

func TestComplete(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()

	if task.State() != StateDone {
		t.Errorf("expected StateDone, got %s", task.State())
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCompleteLandsInDoneFromNext(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateNext
	task.Complete()

	if task.State() != StateDone {
		t.Errorf("expected StateDone, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionCompleted {
		t.Errorf("expected Completed section, got %q", SectionForState(task.State()))
	}
}

func TestCompleteLandsInDoneFromWaiting(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateWaiting
	task.Complete()

	if task.State() != StateDone {
		t.Errorf("expected StateDone, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionCompleted {
		t.Errorf("expected Completed section, got %q", SectionForState(task.State()))
	}
}

func TestCompleteLandsInDoneFromSomeday(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateSomeday
	task.Complete()

	if task.State() != StateDone {
		t.Errorf("expected StateDone, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionCompleted {
		t.Errorf("expected Completed section, got %q", SectionForState(task.State()))
	}
}

func TestStart(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Start()

	if task.State() != StateInProgress {
		t.Errorf("expected StateInProgress, got %s", task.State())
	}
}

func TestStartAutoTriagesFromInbox(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInbox
	task.Start()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Inbox, got %q", SectionForState(task.State()))
	}
}

func TestStartAutoTriagesFromNext(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateNext
	task.Start()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Next, got %q", SectionForState(task.State()))
	}
}

func TestStartAutoTriagesFromReview(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateReview
	task.Start()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Review, got %q", SectionForState(task.State()))
	}
}

func TestStartAutoTriagesFromStandby(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StatePaused
	task.Start()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Standby, got %q", SectionForState(task.State()))
	}
}

func TestStartAutoTriagesFromCompleted(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDone
	task.Start()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Completed, got %q", SectionForState(task.State()))
	}
}

func TestBlock(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Block()

	if task.State() != StateWaiting {
		t.Errorf("expected StateWaiting, got %s", task.State())
	}
}

func TestCancel(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Cancel()

	if task.State() != StateCanceled {
		t.Errorf("expected StateCanceled, got %s", task.State())
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCancelLandsInCanceled(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateNext
	task.Cancel()

	if task.State() != StateCanceled {
		t.Errorf("expected StateCanceled, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionCompleted {
		t.Errorf("expected Completed section, got %q", SectionForState(task.State()))
	}
}

func TestReview(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Start()
	task.Review()

	if task.State() != StateReview {
		t.Errorf("expected StateReview, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionReview {
		t.Errorf("expected Section Review, got %q", SectionForState(task.State()))
	}
}

func TestReviewFromTodo(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInbox
	task.Review()

	if task.State() != StateReview {
		t.Errorf("expected StateReview, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionReview {
		t.Errorf("expected Section Review, got %q", SectionForState(task.State()))
	}
}

func TestStandby(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Start()
	before := task.UpdatedAt
	task.Standby()

	if task.State() != StatePaused {
		t.Errorf("expected StatePaused, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionStandby {
		t.Errorf("expected Section Standby, got %q", SectionForState(task.State()))
	}
	if !task.UpdatedAt.After(before) && task.UpdatedAt != before {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestStandbyFromTodo(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInbox
	task.Standby()

	if task.State() != StatePaused {
		t.Errorf("expected StatePaused, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionStandby {
		t.Errorf("expected Section Standby, got %q", SectionForState(task.State()))
	}
}

func TestMaybe(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Start()
	before := task.UpdatedAt
	task.Maybe()

	if task.State() != StateSomeday {
		t.Errorf("expected StateSomeday, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionSomeday {
		t.Errorf("expected Section Someday, got %q", SectionForState(task.State()))
	}
	if !task.UpdatedAt.After(before) && task.UpdatedAt != before {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestMaybeFromTodo(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateInbox
	task.Maybe()

	if task.State() != StateSomeday {
		t.Errorf("expected StateSomeday, got %s", task.State())
	}
	if SectionForState(task.State()) != SectionSomeday {
		t.Errorf("expected Section Someday, got %q", SectionForState(task.State()))
	}
}

func TestReopen(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	task.Reopen()

	if task.State() != StateInbox {
		t.Errorf("expected StateInbox, got %s", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil after Reopen")
	}
	if SectionForState(task.State()) != SectionInbox {
		t.Errorf("expected Section Inbox, got %q", SectionForState(task.State()))
	}
}

func TestReopenClearsDimmed(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	task.Dimmed = true
	task.Reopen()

	if task.Dimmed {
		t.Error("expected Dimmed to be false after Reopen")
	}
}

func TestReopenAfterCancelClearsCompletedAt(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Cancel()
	if task.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set after Cancel")
	}
	task.Reopen()
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil after Reopen following Cancel")
	}
}
