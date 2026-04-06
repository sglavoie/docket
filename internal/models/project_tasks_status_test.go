package models

import "testing"

func TestProjectGetTasksByStatus_AllStatuses(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	todoTask, err := NewTask("todo", 1, TaskTypeTask, "Todo task")
	if err != nil {
		t.Fatal(err)
	}
	todoTask.Status = StatusTodo

	inProgressTask, err := NewTask("in-progress", 2, TaskTypeTask, "In progress task")
	if err != nil {
		t.Fatal(err)
	}
	inProgressTask.Status = StatusInProgress

	doneTask, err := NewTask("done", 3, TaskTypeTask, "Done task")
	if err != nil {
		t.Fatal(err)
	}
	doneTask.Status = StatusDone

	blockedTask, err := NewTask("blocked", 4, TaskTypeTask, "Blocked task")
	if err != nil {
		t.Fatal(err)
	}
	blockedTask.Status = StatusBlocked

	canceledTask, err := NewTask("canceled", 5, TaskTypeTask, "Canceled task")
	if err != nil {
		t.Fatal(err)
	}
	canceledTask.Status = StatusCanceled

	project.Tasks = []*Task{todoTask, inProgressTask, doneTask, blockedTask, canceledTask}

	tests := []struct {
		status   TaskStatus
		expected int
	}{
		{StatusTodo, 1},
		{StatusInProgress, 1},
		{StatusDone, 1},
		{StatusBlocked, 1},
		{StatusCanceled, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			tasks := project.GetTasksByStatus(tt.status)
			if len(tasks) != tt.expected {
				t.Errorf("GetTasksByStatus(%s) returned %d tasks, want %d", tt.status, len(tasks), tt.expected)
			}
			if len(tasks) > 0 && tasks[0].Status != tt.status {
				t.Errorf("returned task has status %s, want %s", tasks[0].Status, tt.status)
			}
		})
	}
}

func TestProjectGetTasksByStatus_MultipleMatches(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Status = StatusTodo

	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Status = StatusTodo

	task3, err := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Status = StatusDone

	project.Tasks = []*Task{task1, task2, task3}

	todoTasks := project.GetTasksByStatus(StatusTodo)
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}
}

func TestProjectGetCompletedTasks(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	todoTask, err := NewTask("todo", 1, TaskTypeTask, "Todo")
	if err != nil {
		t.Fatal(err)
	}
	todoTask.Status = StatusTodo

	inProgressTask, err := NewTask("in-progress", 2, TaskTypeTask, "In Progress")
	if err != nil {
		t.Fatal(err)
	}
	inProgressTask.Status = StatusInProgress

	doneTask, err := NewTask("done", 3, TaskTypeTask, "Done")
	if err != nil {
		t.Fatal(err)
	}
	doneTask.Status = StatusDone

	canceledTask, err := NewTask("canceled", 4, TaskTypeTask, "Canceled")
	if err != nil {
		t.Fatal(err)
	}
	canceledTask.Status = StatusCanceled

	blockedTask, err := NewTask("blocked", 5, TaskTypeTask, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	blockedTask.Status = StatusBlocked

	project.Tasks = []*Task{todoTask, inProgressTask, doneTask, canceledTask, blockedTask}

	completed := project.GetCompletedTasks()
	if len(completed) != 2 {
		t.Errorf("expected 2 completed tasks (done + canceled), got %d", len(completed))
	}

	for _, task := range completed {
		if task.Status != StatusDone && task.Status != StatusCanceled {
			t.Errorf("completed task has unexpected status: %s", task.Status)
		}
	}
}