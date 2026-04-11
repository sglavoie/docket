package models

import (
	"testing"
	"time"
)

func TestTaskDependencies(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	blocker1, err := NewTask("blocker1", 2, TaskTypeTask, "Blocker 1")
	if err != nil {
		t.Fatal(err)
	}
	blocker2, err := NewTask("blocker2", 3, TaskTypeTask, "Blocker 2")
	if err != nil {
		t.Fatal(err)
	}

	// Initially no dependencies
	if len(task.DependsOn) != 0 {
		t.Errorf("expected 0 dependencies initially, got %d", len(task.DependsOn))
	}
	if task.HasDependency(blocker1.ID) {
		t.Error("task should not have dependency on blocker1 initially")
	}

	// Add dependency
	_ = task.AddDependency(blocker1.ID)
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after add, got %d", len(task.DependsOn))
	}
	if !task.HasDependency(blocker1.ID) {
		t.Error("task should have dependency on blocker1 after add")
	}

	// Add duplicate dependency (should not add)
	_ = task.AddDependency(blocker1.ID)
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after duplicate add, got %d", len(task.DependsOn))
	}

	// Add second dependency
	_ = task.AddDependency(blocker2.ID)
	if len(task.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
	}
	if !task.HasDependency(blocker2.ID) {
		t.Error("task should have dependency on blocker2")
	}

	// Remove dependency
	task.RemoveDependency(blocker1.ID)
	if task.HasDependency(blocker1.ID) {
		t.Error("task should not have dependency on blocker1 after removal")
	}
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after removal, got %d", len(task.DependsOn))
	}

	// Remove non-existent dependency (should not error)
	task.RemoveDependency("nonexistent")
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after removing non-existent, got %d", len(task.DependsOn))
	}

	// Clear dependencies
	task.ClearDependencies()
	if len(task.DependsOn) != 0 {
		t.Errorf("expected 0 dependencies after clear, got %d", len(task.DependsOn))
	}
}

func TestAddDependencyRejectsSelfDependency(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}

	err = task.AddDependency(task.ID)
	if err == nil {
		t.Error("expected error when adding self-dependency, got nil")
	}
	if len(task.DependsOn) != 0 {
		t.Errorf("expected 0 dependencies after self-dependency rejection, got %d", len(task.DependsOn))
	}
}

func TestTaskDependencyUpdatesTimestamp(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	_ = task.AddDependency("blocker-id")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after AddDependency()")
	}

	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.RemoveDependency("blocker-id")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after RemoveDependency()")
	}

	_ = task.AddDependency("another-blocker")
	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.ClearDependencies()
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after ClearDependencies()")
	}
}
