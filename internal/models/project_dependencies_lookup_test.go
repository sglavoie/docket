package models

import "testing"

func TestProjectGetBlockingTasks(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	task1 := newDependencyTestTask(t, "1", 1, "Task 1")
	task2 := newDependencyTestTask(t, "2", 2, "Task 2")
	task3 := newDependencyTestTask(t, "3", 3, "Task 3")

	_ = task3.AddDependency(task1.ID)
	_ = task3.AddDependency(task2.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blockers := project.GetBlockingTasks(task3)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blocking tasks, got %d", len(blockers))
	}

	blockers = project.GetBlockingTasks(task1)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blocking tasks for task1, got %d", len(blockers))
	}
}

func TestProjectGetBlockedByTasks(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	task1 := newDependencyTestTask(t, "1", 1, "Task 1")
	task2 := newDependencyTestTask(t, "2", 2, "Task 2")
	task3 := newDependencyTestTask(t, "3", 3, "Task 3")

	_ = task2.AddDependency(task1.ID)
	_ = task3.AddDependency(task1.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blocked := project.GetBlockedByTasks(task1)
	if len(blocked) != 2 {
		t.Errorf("expected task1 to block 2 tasks, got %d", len(blocked))
	}

	blocked = project.GetBlockedByTasks(task3)
	if len(blocked) != 0 {
		t.Errorf("expected task3 to block 0 tasks, got %d", len(blocked))
	}
}