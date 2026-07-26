package models

import (
	"testing"
	"time"
)

func TestSetSection_GTD(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(SectionNext)
	if !task.InSection(SectionNext) {
		t.Errorf("expected section=%q, got %q", SectionNext, SectionForState(task.State()))
	}
}

func TestSetSection_Empty(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(SectionNext)
	task.SetSection("")
	if !task.InSection("") {
		t.Errorf("expected section empty, got %q", SectionForState(task.State()))
	}
}

func TestSetSection_NonGTDIgnored(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection("Sprint-1")
	if !task.InSection("") {
		t.Errorf("expected section unchanged (empty), got %q", SectionForState(task.State()))
	}
}

func TestSetSection_RefreshesUpdatedAtOnChange(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-time.Hour)
	task.UpdatedAt = stale

	task.SetSection(SectionNext)

	if !task.UpdatedAt.After(stale) {
		t.Error("SetSection should refresh UpdatedAt when section membership changes")
	}
}

func TestSetSectionDelegatesLifecycleSideEffects(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Complete()
	snoozedUntil := time.Now().Add(24 * time.Hour)
	task.SnoozedUntil = &snoozedUntil

	task.SetSection(SectionNext)

	if task.State() != StateNext {
		t.Errorf("State() = %q, want next", task.State())
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
	}
	if task.SnoozedUntil != nil {
		t.Errorf("SnoozedUntil = %v, want nil", task.SnoozedUntil)
	}
}

func TestSetSection_RejectedNamePreservesUpdatedAt(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection("Sprint-1")
	stale := time.Now().Add(-time.Hour)
	task.UpdatedAt = stale

	task.SetSection("bad name")

	if !task.UpdatedAt.Equal(stale) {
		t.Error("SetSection should not refresh UpdatedAt when rejecting an invalid section")
	}
}

func TestNewTaskHasEmptySection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}

	// New tasks start in Active view (empty section)
	if !task.InSection("") {
		t.Errorf("expected new task section to be empty (Active view), got %s", SectionForState(task.State()))
	}
}

func TestTaskCompleteMovesToCompletedFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateNext) // Explicitly place in Next

	if !task.InSection(SectionNext) {
		t.Fatalf("precondition failed: task should be in Next, got %s", SectionForState(task.State()))
	}

	task.Complete()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed after Complete(), got %s", SectionForState(task.State()))
	}
	if task.State() != StateDone {
		t.Errorf("expected state to be done, got %s", task.State())
	}
}

func TestTaskCompleteMovesToCompletedFromActive(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateReady) // Active section

	task.Complete()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed, got %s", SectionForState(task.State()))
	}
}

func TestTaskStartAutoTriageFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateNext) // Explicitly place in Next

	if !task.InSection(SectionNext) {
		t.Fatalf("precondition failed: task should be in Next, got %s", SectionForState(task.State()))
	}

	task.Start()

	if !task.InSection("") {
		t.Errorf("expected section to be cleared after Start(), got %s", SectionForState(task.State()))
	}
	if task.State() != StateInProgress {
		t.Errorf("expected state to be in_progress, got %s", task.State())
	}
}

func TestTaskStartClearsAnySection(t *testing.T) {
	// Start lands in the in_progress state, which lives in the active bucket;
	// the section is cleared unconditionally.
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateSomeday)

	task.Start()

	if !task.InSection("") {
		t.Errorf("expected section cleared, got %s", SectionForState(task.State()))
	}
	if task.State() != StateInProgress {
		t.Errorf("expected in_progress state, got %s", task.State())
	}
}

func TestTaskCancelMovesToCompletedFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateNext) // Explicitly place in Next

	if !task.InSection(SectionNext) {
		t.Fatalf("precondition failed: task should be in Next, got %s", SectionForState(task.State()))
	}

	task.Cancel()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed after Cancel(), got %s", SectionForState(task.State()))
	}
	if task.State() != StateCanceled {
		t.Errorf("expected state to be canceled, got %s", task.State())
	}
}

func TestTaskCancelMovesToCompletedFromActiveSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateReady)

	task.Cancel()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed, got %s", SectionForState(task.State()))
	}
}

func TestTaskBlockMovesToWaiting(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateNext) // Explicitly place in Next

	if !task.InSection(SectionNext) {
		t.Fatalf("precondition failed: task should be in Next, got %s", SectionForState(task.State()))
	}

	task.Block()

	if !task.InSection(SectionWaiting) {
		t.Errorf("expected section to be Waiting after Block(), got %s", SectionForState(task.State()))
	}
	if task.State() != StateWaiting {
		t.Errorf("expected state to be waiting, got %s", task.State())
	}
}

func TestTaskReopenMovesToInbox(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateDone) // Already triaged out of Next

	task.Reopen()

	// Reopen moves task back to Inbox for re-processing
	if !task.InSection(SectionInbox) {
		t.Errorf("expected section to be Inbox after Reopen(), got %s", SectionForState(task.State()))
	}
	if task.State() != StateInbox {
		t.Errorf("expected state to be inbox, got %s", task.State())
	}
}

func TestTaskWithEmptySectionMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateReady) // Active section

	task.Complete()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed, got %s", SectionForState(task.State()))
	}
}

func TestTaskCompleteFromWaitingMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateWaiting)

	task.Complete()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed after Complete() from Waiting, got %s", SectionForState(task.State()))
	}
}

func TestTaskCompleteFromSomedayMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateSomeday)

	task.Complete()

	if !task.InSection(SectionCompleted) {
		t.Errorf("expected section to be Completed after Complete() from Someday, got %s", SectionForState(task.State()))
	}
}

func TestTaskStartAutoTriageFromWaiting(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateWaiting)

	task.Start()

	if !task.InSection("") {
		t.Errorf("expected section to be cleared after Start() from Waiting, got %s", SectionForState(task.State()))
	}
}

func TestTaskStartAutoTriageFromSomeday(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateSomeday)

	task.Start()

	if !task.InSection("") {
		t.Errorf("expected section to be cleared after Start() from Someday, got %s", SectionForState(task.State()))
	}
}

func TestInSection_GTDSection(t *testing.T) {
	task, _ := NewTask("id", 1, TaskTypeTask, "T")
	task.SetSection(SectionNext)

	if !task.InSection(SectionNext) {
		t.Error("expected InSection(Next) = true")
	}
	if task.InSection(SectionInbox) {
		t.Error("expected InSection(Inbox) = false")
	}
	if task.InSection("Sprint-1") {
		t.Error("expected InSection(Sprint-1) = false")
	}
}

func TestInSection_NonGTDSectionIgnored(t *testing.T) {
	task, _ := NewTask("id", 1, TaskTypeTask, "T")
	task.SetSection("Sprint-1")

	if task.InSection("Sprint-1") {
		t.Error("expected InSection(Sprint-1) = false for non-GTD section")
	}
	if task.InSection(SectionNext) {
		t.Error("expected InSection(Next) = false")
	}
	if !task.InSection("") {
		t.Error("expected InSection('') = true when non-GTD section is ignored")
	}
}

func TestInSection_ActiveView(t *testing.T) {
	task, _ := NewTask("id", 1, TaskTypeTask, "T")
	task.SetSection("") // Active view

	if !task.InSection("") {
		t.Error("expected InSection('') = true for Active view task")
	}
	if task.InSection("Sprint-1") {
		t.Error("expected InSection(Sprint-1) = false for Active view task")
	}
}
