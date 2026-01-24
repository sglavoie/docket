package models

import "testing"

func TestProjectGetRootTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task := NewTask("task", 2, TaskTypeTask, "Task")
	childTask := NewTask("child", 3, TaskTypeTask, "Child")

	project.Tasks = []*Task{epic, task, childTask}
	epic.AddChild(childTask)

	roots := project.GetRootTasks()

	if len(roots) != 2 {
		t.Errorf("expected 2 root tasks, got %d", len(roots))
	}

	// Check that childTask is not in roots
	for _, root := range roots {
		if root.ID == "child" {
			t.Error("child task should not be in root tasks")
		}
	}
}

func TestProjectGetTaskByID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")

	project.Tasks = []*Task{task1, task2}

	// Find existing task
	found := project.GetTaskByID("task-1")
	if found == nil {
		t.Fatal("expected to find task-1")
	}
	if found.Title != "Task 1" {
		t.Errorf("expected Title 'Task 1', got '%s'", found.Title)
	}

	// Find non-existent task
	notFound := project.GetTaskByID("task-999")
	if notFound != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestProjectGetTaskByShortID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("task-2", 42, TaskTypeTask, "Task 2")

	project.Tasks = []*Task{task1, task2}

	// Find by short ID
	found := project.GetTaskByShortID(42)
	if found == nil {
		t.Fatal("expected to find task with short ID 42")
	}
	if found.Title != "Task 2" {
		t.Errorf("expected Title 'Task 2', got '%s'", found.Title)
	}

	// Non-existent short ID
	notFound := project.GetTaskByShortID(999)
	if notFound != nil {
		t.Error("expected nil for non-existent short ID")
	}
}

func TestProjectBuildTaskTree(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task}

	project.BuildTaskTree()

	// Check parent-child relationships
	if len(epic.Children) != 1 {
		t.Errorf("expected epic to have 1 child, got %d", len(epic.Children))
	}
	if story.Parent == nil || story.Parent.ID != "epic" {
		t.Error("story's parent should be epic")
	}
	if task.Parent == nil || task.Parent.ID != "story" {
		t.Error("task's parent should be story")
	}

	// Check levels
	if epic.Level != 0 {
		t.Errorf("epic level should be 0, got %d", epic.Level)
	}
	if story.Level != 1 {
		t.Errorf("story level should be 1, got %d", story.Level)
	}
	if task.Level != 2 {
		t.Errorf("task level should be 2, got %d", task.Level)
	}
}

func TestProjectGetNextShortID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

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

func TestProjectGetTaskAbove(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	task2 := NewTask("task2", 4, TaskTypeTask, "Task 2")

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task, task2}
	project.BuildTaskTree()

	// Visual order: epic, story, task, task2
	// Task above epic should be nil
	above := project.GetTaskAbove(epic)
	if above != nil {
		t.Errorf("expected nil above epic, got %s", above.ID)
	}

	// Task above story should be epic
	above = project.GetTaskAbove(story)
	if above == nil || above.ID != "epic" {
		t.Error("expected epic above story")
	}

	// Task above task should be story
	above = project.GetTaskAbove(task)
	if above == nil || above.ID != "story" {
		t.Error("expected story above task")
	}

	// Task above task2 (root) should be task (last in epic's tree)
	above = project.GetTaskAbove(task2)
	if above == nil || above.ID != "task" {
		t.Errorf("expected task above task2, got %v", above)
	}
}

func TestProjectFilterTasks_WithPredicates(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "High priority task")
	task1.Priority = 1

	task2 := NewTask("task-2", 2, TaskTypeTask, "Medium priority task")
	task2.Priority = 2

	task3 := NewTask("task-3", 3, TaskTypeTask, "Low priority task")
	task3.Priority = 3

	task4 := NewTask("task-4", 4, TaskTypeBug, "Bug fix")
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
		// "High priority task", "Medium priority task", "Low priority task" have > 10 chars
		// "Bug fix" has 7 chars
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

func TestProjectGetTasksByStatus_AllStatuses(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	todoTask := NewTask("todo", 1, TaskTypeTask, "Todo task")
	todoTask.Status = StatusTodo

	inProgressTask := NewTask("in-progress", 2, TaskTypeTask, "In progress task")
	inProgressTask.Status = StatusInProgress

	doneTask := NewTask("done", 3, TaskTypeTask, "Done task")
	doneTask.Status = StatusDone

	blockedTask := NewTask("blocked", 4, TaskTypeTask, "Blocked task")
	blockedTask.Status = StatusBlocked

	canceledTask := NewTask("canceled", 5, TaskTypeTask, "Canceled task")
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
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task1.Status = StatusTodo

	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	task2.Status = StatusTodo

	task3 := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	task3.Status = StatusDone

	project.Tasks = []*Task{task1, task2, task3}

	todoTasks := project.GetTasksByStatus(StatusTodo)
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}
}

func TestProjectGetTasksByType_AllTypes(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")
	idea := NewTask("idea", 5, TaskTypeIdea, "Idea")
	note := NewTask("note", 6, TaskTypeNote, "Note")

	project.Tasks = []*Task{epic, story, task, bug, idea, note}

	tests := []struct {
		taskType TaskType
		expected int
	}{
		{TaskTypeEpic, 1},
		{TaskTypeStory, 1},
		{TaskTypeTask, 1},
		{TaskTypeBug, 1},
		{TaskTypeIdea, 1},
		{TaskTypeNote, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			tasks := project.GetTasksByType(tt.taskType)
			if len(tasks) != tt.expected {
				t.Errorf("GetTasksByType(%s) returned %d tasks, want %d", tt.taskType, len(tasks), tt.expected)
			}
			if len(tasks) > 0 && tasks[0].Type != tt.taskType {
				t.Errorf("returned task has type %s, want %s", tasks[0].Type, tt.taskType)
			}
		})
	}
}

func TestProjectGetTasksByType_MultipleMatches(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")

	project.Tasks = []*Task{task1, task2, task3, bug}

	tasks := project.GetTasksByType(TaskTypeTask)
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestProjectGetTasksByTag_SingleTag(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Backend task")
	task1.Tags = []string{"backend", "api"}

	task2 := NewTask("task-2", 2, TaskTypeTask, "Frontend task")
	task2.Tags = []string{"frontend", "ui"}

	task3 := NewTask("task-3", 3, TaskTypeTask, "Full stack task")
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
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task1.Tags = []string{"backend"}

	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	task2.Tags = []string{"frontend"}

	project.Tasks = []*Task{task1, task2}

	nonexistentTasks := project.GetTasksByTag("database")
	if len(nonexistentTasks) != 0 {
		t.Errorf("expected 0 tasks with nonexistent tag, got %d", len(nonexistentTasks))
	}
}

func TestProjectGetTasksByTag_TasksWithNoTags(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task1.Tags = []string{"backend"}

	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	// task2 has no tags (empty slice from NewTask)

	task3 := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	task3.Tags = nil // explicit nil

	project.Tasks = []*Task{task1, task2, task3}

	backendTasks := project.GetTasksByTag("backend")
	if len(backendTasks) != 1 {
		t.Errorf("expected 1 backend task, got %d", len(backendTasks))
	}
}

func TestProjectGetCompletedTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	todoTask := NewTask("todo", 1, TaskTypeTask, "Todo")
	todoTask.Status = StatusTodo

	inProgressTask := NewTask("in-progress", 2, TaskTypeTask, "In Progress")
	inProgressTask.Status = StatusInProgress

	doneTask := NewTask("done", 3, TaskTypeTask, "Done")
	doneTask.Status = StatusDone

	canceledTask := NewTask("canceled", 4, TaskTypeTask, "Canceled")
	canceledTask.Status = StatusCanceled

	blockedTask := NewTask("blocked", 5, TaskTypeTask, "Blocked")
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

func TestProjectFilterMethods_EmptyTaskList(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")
	project.Tasks = []*Task{}

	t.Run("FilterTasks on empty list", func(t *testing.T) {
		result := project.FilterTasks(func(t *Task) bool { return true })
		if result != nil && len(result) != 0 {
			t.Errorf("FilterTasks on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByStatus on empty list", func(t *testing.T) {
		result := project.GetTasksByStatus(StatusTodo)
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByStatus on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByType on empty list", func(t *testing.T) {
		result := project.GetTasksByType(TaskTypeTask)
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByType on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByTag on empty list", func(t *testing.T) {
		result := project.GetTasksByTag("backend")
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByTag on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetCompletedTasks on empty list", func(t *testing.T) {
		result := project.GetCompletedTasks()
		if result != nil && len(result) != 0 {
			t.Errorf("GetCompletedTasks on empty list should return empty, got %d", len(result))
		}
	})

	t.Run("GetRootTasks on empty list", func(t *testing.T) {
		result := project.GetRootTasks()
		if result != nil && len(result) != 0 {
			t.Errorf("GetRootTasks on empty list should return empty, got %d", len(result))
		}
	})
}

func TestProjectFilterMethods_NilTaskList(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")
	project.Tasks = nil

	t.Run("FilterTasks on nil list", func(t *testing.T) {
		result := project.FilterTasks(func(t *Task) bool { return true })
		if result != nil && len(result) != 0 {
			t.Errorf("FilterTasks on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByStatus on nil list", func(t *testing.T) {
		result := project.GetTasksByStatus(StatusTodo)
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByStatus on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByType on nil list", func(t *testing.T) {
		result := project.GetTasksByType(TaskTypeTask)
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByType on nil list should return empty, got %d", len(result))
		}
	})

	t.Run("GetTasksByTag on nil list", func(t *testing.T) {
		result := project.GetTasksByTag("backend")
		if result != nil && len(result) != 0 {
			t.Errorf("GetTasksByTag on nil list should return empty, got %d", len(result))
		}
	})
}
