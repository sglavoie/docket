package models

import "testing"

func TestNewTaskHasEmptySection(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")

	// New tasks start in Active view (empty section)
	if task.Section != "" {
		t.Errorf("expected new task section to be empty (Active view), got %s", task.Section)
	}
}

func TestTaskCompleteMovesToCompletedFromNext(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
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
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "" // Active section

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromNext(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
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
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "Active"

	task.Start()

	if task.Section != "Active" {
		t.Errorf("expected section to remain Active, got %s", task.Section)
	}
}

func TestTaskCancelMovesToCompletedFromNext(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
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
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "CustomSection"

	task.Cancel()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskBlockDoesNotAutoTriage(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionNext // Explicitly place in Next

	if task.Section != SectionNext {
		t.Fatalf("precondition failed: task should be in Next, got %s", task.Section)
	}

	task.Block()

	// Block does not auto-triage - task stays in Next
	if task.Section != SectionNext {
		t.Errorf("expected section to remain Next after Block(), got %s", task.Section)
	}
	if task.Status != StatusBlocked {
		t.Errorf("expected status to be blocked, got %s", task.Status)
	}
}

func TestTaskReopenMovesToInbox(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
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
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "" // Active section

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed, got %s", task.Section)
	}
}

func TestTaskCompleteFromWaitingMovesToCompleted(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionWaiting

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Complete() from Waiting, got %s", task.Section)
	}
}

func TestTaskCompleteFromSomedayMovesToCompleted(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionSomeday

	task.Complete()

	if task.Section != SectionCompleted {
		t.Errorf("expected section to be Completed after Complete() from Someday, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromWaiting(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionWaiting

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start() from Waiting, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromSomeday(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionSomeday

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start() from Someday, got %s", task.Section)
	}
}
