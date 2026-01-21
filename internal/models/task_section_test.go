package models

import "testing"

func TestNewTaskHasEmptySection(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")

	// New tasks start in Active view (empty section)
	if task.Section != "" {
		t.Errorf("expected new task section to be empty (Active view), got %s", task.Section)
	}
}

func TestTaskCompleteAutoTriageFromBacklog(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionBacklog // Explicitly place in Backlog

	if task.Section != SectionBacklog {
		t.Fatalf("precondition failed: task should be in Backlog, got %s", task.Section)
	}

	task.Complete()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Complete(), got %s", task.Section)
	}
	if task.Status != StatusDone {
		t.Errorf("expected status to be done, got %s", task.Status)
	}
}

func TestTaskCompletePreservesNonBacklogSection(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "Active"

	task.Complete()

	if task.Section != "Active" {
		t.Errorf("expected section to remain Active, got %s", task.Section)
	}
}

func TestTaskStartAutoTriageFromBacklog(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionBacklog // Explicitly place in Backlog

	if task.Section != SectionBacklog {
		t.Fatalf("precondition failed: task should be in Backlog, got %s", task.Section)
	}

	task.Start()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Start(), got %s", task.Section)
	}
	if task.Status != StatusInProgress {
		t.Errorf("expected status to be in_progress, got %s", task.Status)
	}
}

func TestTaskStartPreservesNonBacklogSection(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "Active"

	task.Start()

	if task.Section != "Active" {
		t.Errorf("expected section to remain Active, got %s", task.Section)
	}
}

func TestTaskCancelAutoTriageFromBacklog(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionBacklog // Explicitly place in Backlog

	if task.Section != SectionBacklog {
		t.Fatalf("precondition failed: task should be in Backlog, got %s", task.Section)
	}

	task.Cancel()

	if task.Section != "" {
		t.Errorf("expected section to be cleared after Cancel(), got %s", task.Section)
	}
	if task.Status != StatusCancelled {
		t.Errorf("expected status to be cancelled, got %s", task.Status)
	}
}

func TestTaskCancelPreservesNonBacklogSection(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "Done"

	task.Cancel()

	if task.Section != "Done" {
		t.Errorf("expected section to remain Done, got %s", task.Section)
	}
}

func TestTaskBlockDoesNotAutoTriage(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = SectionBacklog // Explicitly place in Backlog

	if task.Section != SectionBacklog {
		t.Fatalf("precondition failed: task should be in Backlog, got %s", task.Section)
	}

	task.Block()

	// Block does not auto-triage - task stays in Backlog
	if task.Section != SectionBacklog {
		t.Errorf("expected section to remain Backlog after Block(), got %s", task.Section)
	}
	if task.Status != StatusBlocked {
		t.Errorf("expected status to be blocked, got %s", task.Status)
	}
}

func TestTaskReopenMovesToInbox(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "" // Already triaged out of Backlog
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

func TestTaskWithEmptySectionNoAutoTriage(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")
	task.Section = "" // Already has empty section

	task.Complete()

	// Should stay empty
	if task.Section != "" {
		t.Errorf("expected section to remain empty, got %s", task.Section)
	}
}
