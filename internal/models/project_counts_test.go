package models

import "testing"

func TestProjectTaskCounts(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1") // todo
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.Start() // in_progress
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")
	task3.Complete() // done
	task4 := NewTask("4", 4, TaskTypeTask, "Task 4")
	task4.Block() // blocked
	task5 := NewTask("5", 5, TaskTypeTask, "Task 5")
	task5.Cancel() // cancelled

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	if project.CountTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", project.CountTasks())
	}

	// Active: todo, in_progress, blocked = 3
	if project.CountActiveTasks() != 3 {
		t.Errorf("expected 3 active tasks, got %d", project.CountActiveTasks())
	}

	// Completed: done, cancelled = 2
	if project.CountCompletedTasks() != 2 {
		t.Errorf("expected 2 completed tasks, got %d", project.CountCompletedTasks())
	}
}

func TestProjectGetTasksByStatus(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.Start()
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	todoTasks := project.GetTasksByStatus(StatusTodo)
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}

	inProgressTasks := project.GetTasksByStatus(StatusInProgress)
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in_progress task, got %d", len(inProgressTasks))
	}
}

func TestProjectGetTasksByType(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("1", 1, TaskTypeEpic, "Epic")
	task1 := NewTask("2", 2, TaskTypeTask, "Task 1")
	task2 := NewTask("3", 3, TaskTypeTask, "Task 2")
	bug := NewTask("4", 4, TaskTypeBug, "Bug")

	project.Tasks = []*Task{epic, task1, task2, bug}

	tasks := project.GetTasksByType(TaskTypeTask)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks of type Task, got %d", len(tasks))
	}

	epics := project.GetTasksByType(TaskTypeEpic)
	if len(epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(epics))
	}
}

func TestProjectGetTasksByTag(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task1.AddTag("backend")
	task1.AddTag("urgent")

	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.AddTag("backend")

	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")
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
