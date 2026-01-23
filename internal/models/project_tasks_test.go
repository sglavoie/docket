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
