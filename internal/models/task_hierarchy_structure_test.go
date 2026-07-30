package models

import "testing"

func TestTaskHierarchy(t *testing.T) {
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := NewGenericTask("story", 2, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task", 3, "Task")
	if err != nil {
		t.Fatal(err)
	}

	// Test IsRoot and IsLeaf
	if !epic.IsRoot() {
		t.Error("epic should be root")
	}
	if !epic.IsLeaf() {
		t.Error("epic should be leaf initially")
	}

	// Add child
	if err := epic.AddChild(story); err != nil {
		t.Fatal(err)
	}
	if story.Parent == nil || story.Parent.ID != epic.ID {
		t.Error("story's parent should be epic")
	}
	if story.IsRoot() {
		t.Error("story should not be root")
	}
	if epic.IsLeaf() {
		t.Error("epic should not be leaf after adding child")
	}
	if story.Level != 1 {
		t.Errorf("story level should be 1, got %d", story.Level)
	}

	// Add grandchild
	if err := story.AddChild(task); err != nil {
		t.Fatal(err)
	}
	if task.Level != 2 {
		t.Errorf("task level should be 2, got %d", task.Level)
	}
}
