package models

import "testing"

func TestAddRelatedToRejectsSelfReference(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	err = task.AddRelatedTo(task.ID)
	if err == nil {
		t.Error("expected error when adding self-reference, got nil")
	}
	if len(task.RelatedTo) != 0 {
		t.Errorf("expected 0 related tasks after self-reference rejection, got %d", len(task.RelatedTo))
	}
}

func TestAddRelatedToAcceptsDifferentTask(t *testing.T) {
	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	err = task1.AddRelatedTo(task2.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !task1.HasRelatedTo(task2.ID) {
		t.Error("expected task1 to be related to task2")
	}
}

// TestAddRelatedToRejectsEmptyID verifies that AddRelatedTo refuses an empty
// ID. Allowing it would silently insert "" into RelatedTo and cause every
// empty-string lookup to match (mirrors AddDependency's empty-blocker guard).
func TestAddRelatedToRejectsEmptyID(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddRelatedTo(""); err == nil {
		t.Fatal("AddRelatedTo(\"\") should return an error")
	}
	if len(task.RelatedTo) != 0 {
		t.Errorf("RelatedTo should remain empty after rejected add, got %v", task.RelatedTo)
	}
}

// TestHasRelatedToEmptyReturnsFalse verifies that HasRelatedTo("") never
// matches, even if a malformed empty entry somehow ended up in RelatedTo.
func TestHasRelatedToEmptyReturnsFalse(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a corrupt on-disk entry bypassing AddRelatedTo.
	task.RelatedTo = []string{""}
	if task.HasRelatedTo("") {
		t.Error("HasRelatedTo(\"\") should return false even if RelatedTo contains an empty string")
	}
	task.RelatedTo = append(task.RelatedTo, "real-id")
	if !task.HasRelatedTo("real-id") {
		t.Error("HasRelatedTo should still return true for a real id")
	}
}
