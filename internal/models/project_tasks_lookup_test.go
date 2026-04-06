package models

import "testing"

func TestProjectGetRootTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task", 2, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	childTask, err := NewTask("child", 3, TaskTypeTask, "Child")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{epic, task, childTask}
	if err := epic.AddChild(childTask); err != nil {
		t.Fatal(err)
	}

	roots := project.GetRootTasks()

	if len(roots) != 2 {
		t.Errorf("expected 2 root tasks, got %d", len(roots))
	}

	for _, root := range roots {
		if root.ID == "child" {
			t.Error("child task should not be in root tasks")
		}
	}
}

func TestProjectGetTaskByID(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}

	found := project.GetTaskByID("task-1")
	if found == nil {
		t.Fatal("expected to find task-1")
	}
	if found.Title != "Task 1" {
		t.Errorf("expected Title 'Task 1', got '%s'", found.Title)
	}

	notFound := project.GetTaskByID("task-999")
	if notFound != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestProjectGetTaskByShortID(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task-2", 42, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2}

	found := project.GetTaskByShortID(42)
	if found == nil {
		t.Fatal("expected to find task with short ID 42")
	}
	if found.Title != "Task 2" {
		t.Errorf("expected Title 'Task 2', got '%s'", found.Title)
	}

	notFound := project.GetTaskByShortID(999)
	if notFound != nil {
		t.Error("expected nil for non-existent short ID")
	}
}

func TestProjectGetNextShortID(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	id1 := project.GetNextShortID()
	if id1 != 1 {
		t.Errorf("first short ID should be 1, got %d", id1)
	}

	id2 := project.GetNextShortID()
	if id2 != 2 {
		t.Errorf("second short ID should be 2, got %d", id2)
	}

	if project.NextShortID != 3 {
		t.Errorf("NextShortID should be 3, got %d", project.NextShortID)
	}
}