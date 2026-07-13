package models

import "testing"

func TestProjectFilterTasks_WithPredicates(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "High priority task")
	if err != nil {
		t.Fatal(err)
	}
	task1.Priority = 1

	task2, err := NewTask("task-2", 2, TaskTypeTask, "Medium priority task")
	if err != nil {
		t.Fatal(err)
	}
	task2.Priority = 2

	task3, err := NewTask("task-3", 3, TaskTypeTask, "Low priority task")
	if err != nil {
		t.Fatal(err)
	}
	task3.Priority = 3

	task4, err := NewTask("task-4", 4, TaskTypeBug, "Bug fix")
	if err != nil {
		t.Fatal(err)
	}
	task4.Priority = 1

	project.Tasks = []*Task{task1, task2, task3, task4}

	t.Run("filter by priority", func(t *testing.T) {
		highPriority := project.FilterTasks(func(t *Task) bool {
			return t.Priority == 1
		})
		if len(highPriority) != 2 {
			t.Errorf("expected 2 high priority tasks, got %d", len(highPriority))
		}
	})

	t.Run("filter by title contains", func(t *testing.T) {
		containing := project.FilterTasks(func(t *Task) bool {
			return len(t.Title) > 10
		})
		if len(containing) != 3 {
			t.Errorf("expected 3 tasks with title > 10 chars, got %d", len(containing))
		}
	})

	t.Run("filter with no matches", func(t *testing.T) {
		none := project.FilterTasks(func(t *Task) bool {
			return t.Priority == 99
		})
		if len(none) != 0 {
			t.Errorf("expected 0 tasks with priority 99, got %d", len(none))
		}
	})

	t.Run("filter all matches", func(t *testing.T) {
		all := project.FilterTasks(func(t *Task) bool {
			return true
		})
		if len(all) != 4 {
			t.Errorf("expected 4 tasks when filter matches all, got %d", len(all))
		}
	})
}

func TestProjectFilterMethods_EmptyTaskList(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{}

	t.Run("FilterTasks on empty list", func(t *testing.T) {
		result := project.FilterTasks(func(t *Task) bool { return true })
		if len(result) != 0 {
			t.Errorf("FilterTasks on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByState on empty list", func(t *testing.T) {
		result := project.GetTasksByState(StateReady)
		if len(result) != 0 {
			t.Errorf("GetTasksByState on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByType on empty list", func(t *testing.T) {
		result := project.GetTasksByType(TaskTypeTask)
		if len(result) != 0 {
			t.Errorf("GetTasksByType on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByTag on empty list", func(t *testing.T) {
		result := project.GetTasksByTag("backend")
		if len(result) != 0 {
			t.Errorf("GetTasksByTag on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetCompletedTasks on empty list", func(t *testing.T) {
		result := project.GetCompletedTasks()
		if len(result) != 0 {
			t.Errorf("GetCompletedTasks on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetRootTasks on empty list", func(t *testing.T) {
		result := project.GetRootTasks()
		if len(result) != 0 {
			t.Errorf("GetRootTasks on empty list should return empty, got %d", len(result))
		}
	})
}

func TestProjectFilterMethods_NilTaskList(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = nil

	t.Run("FilterTasks on nil list", func(t *testing.T) {
		result := project.FilterTasks(func(t *Task) bool { return true })
		if len(result) != 0 {
			t.Errorf("FilterTasks on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByState on nil list", func(t *testing.T) {
		result := project.GetTasksByState(StateReady)
		if len(result) != 0 {
			t.Errorf("GetTasksByState on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByType on nil list", func(t *testing.T) {
		result := project.GetTasksByType(TaskTypeTask)
		if len(result) != 0 {
			t.Errorf("GetTasksByType on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByTag on nil list", func(t *testing.T) {
		result := project.GetTasksByTag("backend")
		if len(result) != 0 {
			t.Errorf("GetTasksByTag on nil list should return empty, got %d", len(result))
		}
	})
}

func TestProjectFilterTasks_NilPredicateReturnsNil(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task-1", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{task}

	result := project.FilterTasks(nil)
	if result != nil {
		t.Errorf("expected nil result for nil predicate, got %v", result)
	}
}
