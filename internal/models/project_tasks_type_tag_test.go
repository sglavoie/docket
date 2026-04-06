package models

import "testing"

func TestProjectGetTasksByType_AllTypes(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := NewTask("story", 2, TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task", 3, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	bug, err := NewTask("bug", 4, TaskTypeBug, "Bug")
	if err != nil {
		t.Fatal(err)
	}
	idea, err := NewTask("idea", 5, TaskTypeIdea, "Idea")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{epic, story, task, bug, idea}

	tests := []struct {
		taskType TaskType
		expected int
	}{
		{TaskTypeEpic, 1},
		{TaskTypeStory, 1},
		{TaskTypeTask, 1},
		{TaskTypeBug, 1},
		{TaskTypeIdea, 1},
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
	task3, err := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	bug, err := NewTask("bug", 4, TaskTypeBug, "Bug")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*Task{task1, task2, task3, bug}

	tasks := project.GetTasksByType(TaskTypeTask)
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestProjectGetTasksByTag_SingleTag(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Backend task")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend", "api"}

	task2, err := NewTask("task-2", 2, TaskTypeTask, "Frontend task")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"frontend", "ui"}

	task3, err := NewTask("task-3", 3, TaskTypeTask, "Full stack task")
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

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend"}

	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
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

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"backend"}

	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	task3, err := NewTask("task-3", 3, TaskTypeTask, "Task 3")
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