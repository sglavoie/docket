package models

import (
	"testing"
)

func TestProjectTaskCounts(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1") // todo
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start() // in_progress
	task3, err := NewGenericTask("3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete() // done
	task4, err := NewGenericTask("4", 4, "Task 4")
	if err != nil {
		t.Fatal(err)
	}
	task4.Wait()
	task5, err := NewGenericTask("5", 5, "Task 5")
	if err != nil {
		t.Fatal(err)
	}
	task5.Cancel() // canceled

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	if project.CountTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", project.CountTasks())
	}

	// Active: todo, doing, waiting = 3
	if project.CountActiveTasks() != 3 {
		t.Errorf("expected 3 active tasks, got %d", project.CountActiveTasks())
	}
}

func TestProjectGetTasksByState(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Start()
	task3, err := NewGenericTask("3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3}

	readyTasks := project.GetTasksByState(StateBacklog)
	if len(readyTasks) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(readyTasks))
	}

	inProgressTasks := project.GetTasksByState(StateDoing)
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in_progress task, got %d", len(inProgressTasks))
	}
}

func TestProjectGetTasksByTag(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.AddTag("backend")
	task1.AddTag("urgent")

	task2, err := NewGenericTask("2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.AddTag("backend")

	task3, err := NewGenericTask("3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.AddTag("frontend")

	project.Tasks = []*Task{task1, task2, task3}

	backend := project.GetTasksByTag("backend")
	if len(backend) != 2 {
		t.Errorf("expected 2 backend tasks, got %d", len(backend))
	}

	urgent := project.GetTasksByTag("urgent")
	if len(urgent) != 1 {
		t.Errorf("expected 1 urgent task, got %d", len(urgent))
	}

	nonexistent := project.GetTasksByTag("nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 tasks with nonexistent tag, got %d", len(nonexistent))
	}
}

func TestEmptyProjectCounts(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	if project.CountTasks() != 0 {
		t.Errorf("expected 0 total tasks, got %d", project.CountTasks())
	}
	if project.CountActiveTasks() != 0 {
		t.Errorf("expected 0 active tasks, got %d", project.CountActiveTasks())
	}
}
