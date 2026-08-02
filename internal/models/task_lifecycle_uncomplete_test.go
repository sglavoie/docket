package models

import "testing"

func TestUncompleteRevertsToInProgress(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	task.Uncomplete()

	if task.State() != StateDoing {
		t.Errorf("expected StateDoing, got %s", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be cleared after Uncomplete")
	}
}

func TestUncompleteLeavesDoneForInProgress(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	if task.State() != StateDone {
		t.Fatalf("expected Completed section after Complete, got %q", task.State())
	}
	task.Uncomplete()

	if task.State() != StateDoing {
		t.Errorf("expected in-progress state, got %q", task.State())
	}
}

func TestUncompleteAlwaysUsesInProgress(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDone
	task.Uncomplete()

	if task.State() != StateDoing {
		t.Errorf("expected in_progress state, got %s", task.State())
	}
}
