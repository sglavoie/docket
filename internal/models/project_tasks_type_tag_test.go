package models

import "testing"

func TestProjectGetTasksByTag_SingleTag(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Backend task")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend", "api"}

	task2, err := NewGenericTask("task-2", 2, "Frontend task")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"frontend", "ui"}

	task3, err := NewGenericTask("task-3", 3, "Full stack task")
	if err != nil {
		t.Fatal(err)
	}
	task3.Tags = []string{"backend", "frontend"}

	project.Tasks = []*Task{task1, task2, task3}

	backendTasks := project.GetTasksByTag("backend")
	if len(backendTasks) != 2 {
		t.Errorf("expected 2 backend tasks, got %d", len(backendTasks))
	}

	frontendTasks := project.GetTasksByTag("frontend")
	if len(frontendTasks) != 2 {
		t.Errorf("expected 2 frontend tasks, got %d", len(frontendTasks))
	}

	apiTasks := project.GetTasksByTag("api")
	if len(apiTasks) != 1 {
		t.Errorf("expected 1 api task, got %d", len(apiTasks))
	}
}

func TestProjectGetTasksByTag_NoMatches(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend"}

	task2, err := NewGenericTask("task-2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"frontend"}

	project.Tasks = []*Task{task1, task2}

	nonexistentTasks := project.GetTasksByTag("database")
	if len(nonexistentTasks) != 0 {
		t.Errorf("expected 0 tasks with nonexistent tag, got %d", len(nonexistentTasks))
	}
}

func TestProjectGetTasksByTag_TasksWithNoTags(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewGenericTask("task-1", 1, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend"}

	task2, err := NewGenericTask("task-2", 2, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	task3, err := NewGenericTask("task-3", 3, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Tags = nil

	project.Tasks = []*Task{task1, task2, task3}

	backendTasks := project.GetTasksByTag("backend")
	if len(backendTasks) != 1 {
		t.Errorf("expected 1 backend task, got %d", len(backendTasks))
	}
}
