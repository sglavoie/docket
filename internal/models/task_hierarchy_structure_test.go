package models

import "testing"

func TestTaskHierarchy(t *testing.T) {
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

func TestTaskCanAddChild(t *testing.T) {
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

	// Epic can have story, task, bug, idea
	if !epic.CanAddChild(TaskTypeStory) {
		t.Error("epic should accept story child")
	}
	if !epic.CanAddChild(TaskTypeTask) {
		t.Error("epic should accept task child")
	}
	if !epic.CanAddChild(TaskTypeBug) {
		t.Error("epic should accept bug child")
	}
	if !epic.CanAddChild(TaskTypeIdea) {
		t.Error("epic should accept idea child")
	}
	if epic.CanAddChild(TaskTypeEpic) {
		t.Error("epic should not accept epic child")
	}

	// Story can have task, bug, idea
	if !story.CanAddChild(TaskTypeTask) {
		t.Error("story should accept task child")
	}
	if !story.CanAddChild(TaskTypeBug) {
		t.Error("story should accept bug child")
	}
	if !story.CanAddChild(TaskTypeIdea) {
		t.Error("story should accept idea child")
	}
	if story.CanAddChild(TaskTypeStory) {
		t.Error("story should not accept story child")
	}

	// Leaf types cannot have children
	if task.CanAddChild(TaskTypeTask) {
		t.Error("task should not accept children")
	}
	if bug.CanAddChild(TaskTypeTask) {
		t.Error("bug should not accept children")
	}
	if idea.CanAddChild(TaskTypeTask) {
		t.Error("idea should not accept children")
	}
}
