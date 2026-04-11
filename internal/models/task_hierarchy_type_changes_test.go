package models

import "testing"

func TestPromoteWithNestedChildren(t *testing.T) {
	// Story with task children can promote to epic (tasks valid under epic)
	story, err := NewTask("story", 1, TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewTask("task1", 2, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 3, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	if err = story.AddChild(task1); err != nil {
		t.Fatal(err)
	}
	if err = story.AddChild(task2); err != nil {
		t.Fatal(err)
	}

	err = story.Promote()
	if err != nil {
		t.Errorf("Promote() story with task children should succeed: %v", err)
	}
	if story.Type != TaskTypeEpic {
		t.Errorf("Type = %s, want epic", story.Type)
	}
	// Children should still be valid
	if len(story.Children) != 2 {
		t.Errorf("expected 2 children after promote, got %d", len(story.Children))
	}
}

func TestDemoteWithNestedChildren(t *testing.T) {
	// Epic with task children can demote to story (tasks valid under story)
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewTask("task1", 2, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	bug, err := NewTask("bug", 3, TaskTypeBug, "Bug")
	if err != nil {
		t.Fatal(err)
	}
	if err = epic.AddChild(task1); err != nil {
		t.Fatal(err)
	}
	if err = epic.AddChild(bug); err != nil {
		t.Fatal(err)
	}

	err = epic.Demote()
	if err != nil {
		t.Errorf("Demote() epic with task/bug children should succeed: %v", err)
	}
	if epic.Type != TaskTypeStory {
		t.Errorf("Type = %s, want story", epic.Type)
	}
	// Children should still be valid
	if len(epic.Children) != 2 {
		t.Errorf("expected 2 children after demote, got %d", len(epic.Children))
	}
}

func TestDemoteEpicWithIdeaChildren(t *testing.T) {
	// Epic with idea children can demote (idea can be child of story)
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	idea, err := NewTask("idea", 2, TaskTypeIdea, "Idea")
	if err != nil {
		t.Fatal(err)
	}
	if err = epic.AddChild(idea); err != nil {
		t.Fatal(err)
	}

	err = epic.Demote()
	if err != nil {
		t.Errorf("Demote() epic with idea child should succeed, got: %v", err)
	}
	// Type should become story
	if epic.Type != TaskTypeStory {
		t.Errorf("Type should become story after demote, got %s", epic.Type)
	}
}
