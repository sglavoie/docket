package models

import "testing"

func TestTaskPromoteWithInvalidHierarchy(t *testing.T) {
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := NewTask("story", 2, TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	if err = epic.AddChild(story); err != nil {
		t.Fatal(err)
	}

	err = epic.Demote()
	if err == nil {
		t.Error("Demote() epic with story child should return error")
	}

	task, err := NewTask("task", 3, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	if err = story.AddChild(task); err != nil {
		t.Fatal(err)
	}

	err = task.Promote()
	if err == nil {
		t.Error("Promote() task under story should return error")
	}
}