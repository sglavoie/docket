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
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
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
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
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
	sourceTask, err := NewTask("source-task", 1, TaskTypeTask, "Source task")
	if err != nil {
		t.Fatal(err)
	}
	externalTask, err := NewTask("external-task", 2, TaskTypeTask, "External task")
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
	sourceTask, err := NewTask("task1", 1, TaskTypeTask, "Source task")
	if err != nil {
		t.Fatal(err)
	}
	duplicatePointer, err := NewTask("task1", 99, TaskTypeTask, "Duplicate pointer")
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

func TestMoveTaskToProjectSelfMoveError(t *testing.T) {
	proj, err := NewProject("proj1", "Project 1", "/tmp/proj1")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
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
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	proj.Tasks = []*Task{task}

	_, err = proj.MoveTaskToProject(task, nil)
	if err == nil {
		t.Fatal("expected error when destination is nil")
	}
}
