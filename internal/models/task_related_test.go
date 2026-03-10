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
