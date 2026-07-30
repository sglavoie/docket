package models

import "testing"

func TestMoveTaskToProjectNilInputs(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}

	result, err := sourceProj.MoveTaskToProject(nil, destProj)
	if err == nil {
		t.Error("expected error for nil task")
	}
	if result != nil {
		t.Error("expected nil result for nil task")
	}

	result, err = sourceProj.MoveTaskToProject(task, nil)
	if err == nil {
		t.Error("expected error for nil destination")
	}
	if result != nil {
		t.Error("expected nil result for nil destination")
	}
}

func TestMoveTaskToProjectSameProject(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{task}

	result, err := sourceProj.MoveTaskToProject(task, sourceProj)
	if err == nil {
		t.Error("expected error when moving to same project")
	}
	if result != nil {
		t.Error("expected nil result when moving to same project")
	}

	if len(sourceProj.Tasks) != 1 {
		t.Error("expected task to remain in project")
	}
}

func TestMoveTaskToProjectRequiresTaskInSourceProject(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}
	sourceTask, err := NewGenericTask("source-task", 1, "Source task")
	if err != nil {
		t.Fatal(err)
	}
	externalTask, err := NewGenericTask("external-task", 2, "External task")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{sourceTask}

	result, err := sourceProj.MoveTaskToProject(externalTask, destProj)
	if err == nil {
		t.Fatal("expected error for task outside source project")
	}
	if result != nil {
		t.Fatal("expected nil result for failed move")
	}
	if len(sourceProj.Tasks) != 1 || sourceProj.Tasks[0] != sourceTask {
		t.Fatal("source project tasks changed after failed move")
	}
	if len(destProj.Tasks) != 0 {
		t.Fatal("destination project received a task from failed move")
	}
}

func TestMoveTaskToProjectRejectsDifferentPointerWithSourceTaskID(t *testing.T) {
	sourceProj, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destProj, err := NewProject("dest", "Dest Project", "/tmp/dest")
	if err != nil {
		t.Fatal(err)
	}
	sourceTask, err := NewGenericTask("task1", 1, "Source task")
	if err != nil {
		t.Fatal(err)
	}
	duplicatePointer, err := NewGenericTask("task1", 99, "Duplicate pointer")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{sourceTask}

	result, err := sourceProj.MoveTaskToProject(duplicatePointer, destProj)
	if err == nil {
		t.Fatal("expected error for non-source pointer with source task ID")
	}
	if result != nil {
		t.Fatal("expected nil result for failed move")
	}
	if len(sourceProj.Tasks) != 1 || sourceProj.Tasks[0] != sourceTask {
		t.Fatal("source project tasks changed after failed move")
	}
	if len(destProj.Tasks) != 0 {
		t.Fatal("destination project received a task from failed move")
	}
}

func TestMoveTaskToProjectRejectsDestinationDescendantIDCollisionWithoutMutation(t *testing.T) {
	source, err := NewProject("source", "Source Project", "/tmp/source")
	if err != nil {
		t.Fatal(err)
	}
	destination, err := NewProject("destination", "Destination Project", "/tmp/destination")
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := NewGenericTask("parent", 1, "Parent")
	child, _ := NewGenericTask("child", 2, "Child")
	child.ParentID = &parent.ID
	collision, _ := NewGenericTask("child", 1, "Collision")
	source.Tasks = []*Task{parent, child}
	destination.Tasks = []*Task{collision}

	result, err := source.MoveTaskToProject(parent, destination)
	if err == nil {
		t.Fatal("expected destination descendant-ID collision")
	}
	if result != nil {
		t.Fatal("failed move returned a result")
	}
	if len(source.Tasks) != 2 || source.Tasks[0] != parent || source.Tasks[1] != child {
		t.Fatal("failed move mutated source task membership or pointers")
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("failed move changed child parent to %v", child.ParentID)
	}
	if len(destination.Tasks) != 1 || destination.Tasks[0] != collision {
		t.Fatal("failed move mutated destination")
	}
}

func TestMoveTaskToProjectSelfMoveError(t *testing.T) {
	proj, err := NewProject("proj1", "Project 1", "/tmp/proj1")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	proj.Tasks = []*Task{task}

	_, err = proj.MoveTaskToProject(task, proj)
	if err == nil {
		t.Fatal("expected error when moving task to same project")
	}
	if err.Error() != "cannot move task to the same project" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMoveTaskToProjectNilTask(t *testing.T) {
	proj, err := NewProject("proj1", "Project 1", "/tmp/proj1")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := NewProject("proj2", "Project 2", "/tmp/proj2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = proj.MoveTaskToProject(nil, dest)
	if err == nil {
		t.Fatal("expected error when moving nil task")
	}
}

func TestMoveTaskToProjectNilDest(t *testing.T) {
	proj, err := NewProject("proj1", "Project 1", "/tmp/proj1")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	proj.Tasks = []*Task{task}

	_, err = proj.MoveTaskToProject(task, nil)
	if err == nil {
		t.Fatal("expected error when destination is nil")
	}
}
