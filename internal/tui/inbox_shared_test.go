package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureInboxSnapshotIncludesStatusFields(t *testing.T) {
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	task := &models.Task{
		ID:            "t1",
		Status:        models.StatusDone,
		CompletedAt:   &completedAt,
		Active:        false,
		Section:       models.SectionCompleted,
		CustomSection: "",
		Priority:      2,
		Tags:          []string{"backend"},
		DueDate:       &dueDate,
		HasTime:       true,
		DependsOn:     []string{"dep-1"},
	}

	snap := captureInboxSnapshot(task)

	if snap.status != models.StatusDone {
		t.Errorf("status: got %q, want %q", snap.status, models.StatusDone)
	}
	if snap.completedAt == nil {
		t.Fatal("completedAt should not be nil")
	}
	if !snap.completedAt.Equal(completedAt) {
		t.Errorf("completedAt: got %v, want %v", *snap.completedAt, completedAt)
	}
	if snap.active != false {
		t.Error("active: got true, want false")
	}
	if snap.section != models.SectionCompleted {
		t.Errorf("section: got %q, want %q", snap.section, models.SectionCompleted)
	}
}

func TestCaptureInboxSnapshotDeepCopiesTimePointers(t *testing.T) {
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	task := &models.Task{
		ID:          "t1",
		Status:      models.StatusDone,
		CompletedAt: &completedAt,
		DueDate:     &dueDate,
		Tags:        []string{},
		DependsOn:   []string{},
	}

	snap := captureInboxSnapshot(task)

	// Mutating original should not affect snapshot
	newTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	task.CompletedAt = &newTime
	task.DueDate = &newTime

	if snap.completedAt.Equal(newTime) {
		t.Error("completedAt should be a deep copy, not a shared pointer")
	}
	if snap.dueDate.Equal(newTime) {
		t.Error("dueDate should be a deep copy, not a shared pointer")
	}
}

func TestRestoreInboxSnapshotRestoresStatusFields(t *testing.T) {
	// Start with a task in "todo" state
	task := &models.Task{
		ID:        "t1",
		Status:    models.StatusTodo,
		Active:    false,
		Section:   models.SectionInbox,
		Priority:  3,
		Tags:      []string{"frontend"},
		DependsOn: []string{},
	}

	// Capture snapshot in todo state
	snap := captureInboxSnapshot(task)

	// Simulate completing the task (as changeTaskStatus does)
	task.ApplyTransition(models.StatusDone)

	// Verify task is now completed
	if task.Status != models.StatusDone {
		t.Fatalf("precondition: task should be done, got %q", task.Status)
	}
	if task.CompletedAt == nil {
		t.Fatal("precondition: CompletedAt should be set after completion")
	}
	if task.Section != models.SectionCompleted {
		t.Fatalf("precondition: section should be Completed, got %q", task.Section)
	}

	// Undo by restoring snapshot
	restoreInboxSnapshot(task, snap)

	// Status should be restored to todo
	if task.Status != models.StatusTodo {
		t.Errorf("status after restore: got %q, want %q", task.Status, models.StatusTodo)
	}
	if task.CompletedAt != nil {
		t.Error("completedAt should be nil after restoring todo snapshot")
	}
	if task.Active != false {
		t.Error("active should be false after restoring todo snapshot")
	}
	if task.Section != models.SectionInbox {
		t.Errorf("section after restore: got %q, want %q", task.Section, models.SectionInbox)
	}
	if task.Priority != 3 {
		t.Errorf("priority after restore: got %d, want 3", task.Priority)
	}
}

func TestRestoreInboxSnapshotRestoresCompletedAt(t *testing.T) {
	// Edge case: restoring a snapshot that HAD a completedAt
	// (e.g., undoing a reopen)
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:          "t1",
		Status:      models.StatusDone,
		CompletedAt: &completedAt,
		Active:      false,
		Section:     models.SectionCompleted,
		Tags:        []string{},
		DependsOn:   []string{},
	}

	snap := captureInboxSnapshot(task)

	// Simulate reopening
	task.Reopen()

	if task.Status != models.StatusTodo {
		t.Fatalf("precondition: reopened task should be todo, got %q", task.Status)
	}

	// Undo the reopen
	restoreInboxSnapshot(task, snap)

	if task.Status != models.StatusDone {
		t.Errorf("status after restore: got %q, want %q", task.Status, models.StatusDone)
	}
	if task.CompletedAt == nil {
		t.Fatal("completedAt should be restored")
	}
	if !task.CompletedAt.Equal(completedAt) {
		t.Errorf("completedAt: got %v, want %v", *task.CompletedAt, completedAt)
	}
}

func TestCaptureInboxSnapshotNilCompletedAt(t *testing.T) {
	task := &models.Task{
		ID:        "t1",
		Status:    models.StatusTodo,
		Tags:      []string{},
		DependsOn: []string{},
	}

	snap := captureInboxSnapshot(task)

	if snap.completedAt != nil {
		t.Error("completedAt should be nil for a todo task")
	}
	if snap.status != models.StatusTodo {
		t.Errorf("status: got %q, want %q", snap.status, models.StatusTodo)
	}
}
