package models

import "testing"

func TestUncompleteRevertsToInProgress(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	task.Uncomplete()

	if task.State() != StateInProgress {
		t.Errorf("expected StateInProgress, got %s", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be cleared after Uncomplete")
	}
}

func TestUncompleteVacatesCompletedSection(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	if SectionForState(task.State()) != SectionCompleted {
		t.Fatalf("expected Completed section after Complete, got %q", SectionForState(task.State()))
	}
	task.Uncomplete()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared from Completed, got %q", SectionForState(task.State()))
	}
}

func TestUncompleteClearsAnySection(t *testing.T) {
	// Uncomplete lands in the in_progress state, which lives in the active
	// bucket — the section is cleared even for non-canonical combinations
	// like a done task still carrying the Next section.
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateDone
	task.Uncomplete()

	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared, got %q", SectionForState(task.State()))
	}
	if task.State() != StateInProgress {
		t.Errorf("expected in_progress state, got %s", task.State())
	}
}
