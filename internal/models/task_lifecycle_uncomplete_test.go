package models

import "testing"

func TestUncompleteRevertsToInProgress(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	task.Uncomplete()

	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress, got %s", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be cleared after Uncomplete")
	}
}

func TestUncompleteVacatesCompletedSection(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	if task.Section != SectionCompleted {
		t.Fatalf("expected Completed section after Complete, got %q", task.Section)
	}
	task.Uncomplete()

	if task.Section != "" {
		t.Errorf("expected section cleared from Completed, got %q", task.Section)
	}
}

func TestUncompletePreservesNonCompletedSection(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Status = StatusDone
	task.Section = SectionNext
	task.Uncomplete()

	if task.Section != SectionNext {
		t.Errorf("expected non-Completed section preserved, got %q", task.Section)
	}
	if task.Status != StatusInProgress {
		t.Errorf("expected StatusInProgress, got %s", task.Status)
	}
}
