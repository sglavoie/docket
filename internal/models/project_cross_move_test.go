package models

import "testing"

func TestMoveTaskToProjectBasic(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if len(result.MovedTasks) != 1 {
		t.Errorf("expected 1 moved task, got %d", len(result.MovedTasks))
	}
	if result.MovedTasks[0].ID != task1.ID {
		t.Errorf("expected task1 to be moved, got %s", result.MovedTasks[0].ID)
	}

	if len(sourceProj.Tasks) != 1 {
		t.Errorf("expected 1 task in source, got %d", len(sourceProj.Tasks))
	}
	if sourceProj.Tasks[0].ID != task2.ID {
		t.Errorf("expected task2 to remain in source, got %s", sourceProj.Tasks[0].ID)
	}

	if len(destProj.Tasks) != 1 {
		t.Errorf("expected 1 task in dest, got %d", len(destProj.Tasks))
	}
	if destProj.Tasks[0].ID != task1.ID {
		t.Errorf("expected task1 in dest, got %s", destProj.Tasks[0].ID)
	}

	if result.OldShortIDs[0] != 1 {
		t.Errorf("expected old short ID 1, got %d", result.OldShortIDs[0])
	}
	if task1.ShortID != 1 {
		t.Errorf("expected new short ID 1 (first in dest), got %d", task1.ShortID)
	}
}

func TestMoveTaskToProjectWithChildren(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewTask("other", 4, TaskTypeTask, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	parentID := parent.ID
	child1.ParentID = &parentID
	child2.ParentID = &parentID

	sourceProj.Tasks = []*Task{parent, child1, child2, other}
	sourceProj.BuildTaskTree()

	result, err := sourceProj.MoveTaskToProject(parent, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if len(result.MovedTasks) != 3 {
		t.Errorf("expected 3 moved tasks (parent + 2 children), got %d", len(result.MovedTasks))
	}

	if len(sourceProj.Tasks) != 1 {
		t.Errorf("expected 1 task in source, got %d", len(sourceProj.Tasks))
	}
	if sourceProj.Tasks[0].ID != other.ID {
		t.Errorf("expected 'other' task to remain in source")
	}

	if len(destProj.Tasks) != 3 {
		t.Errorf("expected 3 tasks in dest, got %d", len(destProj.Tasks))
	}

	if parent.ParentID != nil {
		t.Error("expected parent's ParentID to be cleared")
	}

	if child1.ParentID == nil || *child1.ParentID != parent.ID {
		t.Error("expected child1 to still reference parent")
	}
	if child2.ParentID == nil || *child2.ParentID != parent.ID {
		t.Error("expected child2 to still reference parent")
	}
}

func TestMoveTaskToProjectRegeneratesShortIDs(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task1", 5, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 10, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if result.OldShortIDs[0] != 5 {
		t.Errorf("expected old short ID 5, got %d", result.OldShortIDs[0])
	}

	if task1.ShortID != 1 {
		t.Errorf("expected new short ID 1, got %d", task1.ShortID)
	}
}

func TestMoveTaskToProjectUpdatesTimestamps(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}

	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{task}
	sourceProj.BuildTaskTree()

	oldTaskTime := task.UpdatedAt
	oldSourceTime := sourceProj.UpdatedAt
	oldDestTime := destProj.UpdatedAt

	_, err = sourceProj.MoveTaskToProject(task, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	if !task.UpdatedAt.After(oldTaskTime) {
		t.Error("expected task timestamp to be updated")
	}
	if !sourceProj.UpdatedAt.After(oldSourceTime) {
		t.Error("expected source project timestamp to be updated")
	}
	if !destProj.UpdatedAt.After(oldDestTime) {
		t.Error("expected dest project timestamp to be updated")
	}
}
