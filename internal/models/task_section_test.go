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
	if task.Section != SectionNext {
		t.Errorf("expected Section=%q, got %q", SectionNext, task.Section)
	}
}

func TestSetSection_Empty(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.SetSection("")
	if task.Section != "" {
		t.Errorf("expected Section empty, got %q", task.Section)
	}
}

func TestSetSection_NonGTDIgnored(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection("Sprint-1")
	if task.Section != "" {
		t.Errorf("expected Section unchanged (empty), got %q", task.Section)
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
	if task.Section != "" {
		t.Errorf("expected new task section to be empty (Active view), got %s", task.Section)
	}
}

func TestTaskCompleteMovesToCompletedFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext // Explicitly place in Next

	if task.Section != SectionNext {
		t.Fatalf("precondition failed: task should be in Next, got %s", task.Section)
	}

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Complete(), got %s", task.Section)
	}
	if task.Status != StatusDone {
		t.Errorf("expected status to be done, got %s", task.Status)
	}
}

func TestTaskCompleteMovesToCompletedFromActive(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "" // Active section

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext // Explicitly place in Next

	if task.Section != SectionNext {
		t.Fatalf("precondition failed: task should be in Next, got %s", task.Section)
	}

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start(), got %s", task.Section)
	}
	if task.Status != StatusInProgress {
		t.Errorf("expected status to be in_progress, got %s", task.Status)
	}
}

func TestTaskStartPreservesNonNextSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "Active"

	task.Start()

	if task.Section != "Active" {
		t.Errorf("expected section to remain Active, got %s", task.Section)
	}
}

func TestTaskCancelMovesToCompletedFromNext(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext // Explicitly place in Next

	if task.Section != SectionNext {
		t.Fatalf("precondition failed: task should be in Next, got %s", task.Section)
	}

	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Cancel(), got %s", task.Section)
	}
	if task.Status != StatusCanceled {
		t.Errorf("expected status to be canceled, got %s", task.Status)
	}
}

func TestTaskCancelMovesToCompletedFromNonGTDSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "LegacyBoard"

	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskBlockMovesToWaiting(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext // Explicitly place in Next

	if task.Section != SectionNext {
		t.Fatalf("precondition failed: task should be in Next, got %s", task.Section)
	}

	task.Block()

	if task.Section != SectionWaiting {
		t.Errorf("expected section to be Waiting after Block(), got %s", task.Section)
	}
	if task.Status != StatusBlocked {
		t.Errorf("expected status to be blocked, got %s", task.Status)
	}
}

func TestTaskReopenMovesToInbox(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "" // Already triaged out of Next
	task.Status = StatusDone

	task.Reopen()

	// Reopen moves task back to Inbox for re-processing
	if task.Section != SectionInbox {
		t.Errorf("expected section to be Inbox after Reopen(), got %s", task.Section)
	}
	if task.Status != StatusTodo {
		t.Errorf("expected status to be todo, got %s", task.Status)
	}
}

func TestTaskWithEmptySectionMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "" // Active section

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskCompleteFromWaitingMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionWaiting

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Complete() from Waiting, got %s", task.Section)
	}
}

func TestTaskCompleteFromSomedayMovesToCompleted(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionSomeday

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Complete() from Someday, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromWaiting(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionWaiting

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start() from Waiting, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromSomeday(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionSomeday

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start() from Someday, got %s", task.Section)
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
