package models

import "testing"

func TestComplete(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionNext
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromWaiting(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionWaiting
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteMovesToCompletedFromSomeday(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionSomeday
	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestCompleteKeepsCustomSection(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.CustomSection = "CustomSection"
	task.Complete()

	if task.CustomSection != "CustomSection" {
		t.Errorf("expected custom section preserved, got %q", task.CustomSection)
	}
}

func TestStart(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Start()

	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress, got %s", task.Status)
	}
}

func TestStartAutoTriagesFromInbox(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionInbox
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Inbox, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromNext(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionNext
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Next, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromReview(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionReview
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Review, got %q", task.Section)
	}
}

func TestStartAutoTriagesFromStandby(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionStandby
	task.Start()

	if task.Section != "" {
		t.Errorf("expected section cleared from Standby, got %q", task.Section)
	}
}

func TestBlock(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Block()

	if task.Status != StatusBlocked {
		t.Errorf("expected StatusBlocked, got %s", task.Status)
	}
}

func TestCancel(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Cancel()

	if task.Status != StatusCanceled {
		t.Errorf("expected StatusCanceled, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCancelMovesToCompletedSection(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionNext
	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %q", task.Section)
	}
}

func TestReview(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
	task.Section = SectionInbox
	task.Standby()

	if task.Status != StatusStandby {
		t.Errorf("expected StatusStandby, got %s", task.Status)
	}
	if task.Section != SectionStandby {
		t.Errorf("expected Section Standby, got %q", task.Section)
	}
}

func TestReopen(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
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
	task := newLifecycleTestTask(t, "t1", 1)
	task.Active = true
	task.Cancel()

	if task.Active {
		t.Error("expected Active to be false after Cancel")
	}
}

func TestReopenClearsActive(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Active = true
	task.Complete()
	task.Reopen()

	if task.Active {
		t.Error("expected Active to be false after Reopen")
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
