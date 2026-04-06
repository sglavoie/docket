package models

import "testing"

func TestProjectBuildTaskTree(t *testing.T) {
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

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task}

	project.BuildTaskTree()

	if len(epic.Children) != 1 {
		t.Errorf("expected epic to have 1 child, got %d", len(epic.Children))
	}
	if story.Parent == nil || story.Parent.ID != "epic" {
		t.Error("story's parent should be epic")
	}
	if task.Parent == nil || task.Parent.ID != "story" {
		t.Error("task's parent should be story")
	}

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

func TestProjectGetTaskAbove(t *testing.T) {
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
	task2, err := NewTask("task2", 4, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task, task2}
	project.BuildTaskTree()

	above := project.GetTaskAbove(epic)
	if above != nil {
		t.Errorf("expected nil above epic, got %s", above.ID)
	}

	above = project.GetTaskAbove(story)
	if above == nil || above.ID != "epic" {
		t.Error("expected epic above story")
	}

	above = project.GetTaskAbove(task)
	if above == nil || above.ID != "story" {
		t.Error("expected story above task")
	}

	above = project.GetTaskAbove(task2)
	if above == nil || above.ID != "task" {
		t.Errorf("expected task above task2, got %v", above)
	}
}