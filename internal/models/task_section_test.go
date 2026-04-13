package models

import "testing"

func TestSetSection_GTD(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(SectionNext)
	if task.Section != SectionNext {
		t.Errorf("expected Section=%q, got %q", SectionNext, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection empty, got %q", task.CustomSection)
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

func TestSetSection_Custom(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection("Sprint-1")
	if task.CustomSection != "Sprint-1" {
		t.Errorf("expected CustomSection=%q, got %q", "Sprint-1", task.CustomSection)
	}
	if task.Section != "" {
		t.Errorf("expected Section unchanged (empty), got %q", task.Section)
	}
}

func TestSetSection_GTDClearsCustom(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"
	task.SetSection(SectionInbox)
	if task.Section != SectionInbox {
		t.Errorf("expected Section=%q, got %q", SectionInbox, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestSetSection_CustomClearsGTD(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.SetSection("Sprint-1")
	if task.CustomSection != "Sprint-1" {
		t.Errorf("expected CustomSection=%q, got %q", "Sprint-1", task.CustomSection)
	}
	if task.Section != "" {
		t.Errorf("expected Section cleared, got %q", task.Section)
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

func TestTaskCancelMovesToCompletedFromCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = "CustomSection"

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

// Tests that lifecycle methods clear CustomSection to prevent dual-section state.

func TestCompleteClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected Section=%q, got %q", SectionCompleted, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestCancelClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected Section=%q, got %q", SectionCompleted, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestBlockClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	task.Block()

	if task.Section != SectionWaiting {
		t.Errorf("expected Section=%q, got %q", SectionWaiting, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestReviewClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	task.Review()

	if task.Section != SectionReview {
		t.Errorf("expected Section=%q, got %q", SectionReview, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestStandbyClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"

	task.Standby()

	if task.Section != SectionStandby {
		t.Errorf("expected Section=%q, got %q", SectionStandby, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestReopenClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.CustomSection = "Sprint-1"
	task.Status = StatusDone

	task.Reopen()

	if task.Section != SectionInbox {
		t.Errorf("expected Section=%q, got %q", SectionInbox, task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}

func TestStartFromNextClearsCustomSection(t *testing.T) {
	task, err := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionNext
	task.CustomSection = "Sprint-1"

	task.Start()

	if task.Section != "" {
		t.Errorf("expected Section cleared, got %q", task.Section)
	}
	if task.CustomSection != "" {
		t.Errorf("expected CustomSection cleared, got %q", task.CustomSection)
	}
}
