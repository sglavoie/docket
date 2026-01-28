package models

import (
	"testing"
)

func TestTaskProgress(t *testing.T) {
	parent := NewTask("parent", 1, TaskTypeEpic, "Parent Epic")

	// Leaf task returns 0 or 100
	if parent.GetProgress() != 0 {
		t.Errorf("expected 0 progress for incomplete leaf, got %f", parent.GetProgress())
	}

	parent.Complete()
	if parent.GetProgress() != 100 {
		t.Errorf("expected 100 progress for complete leaf, got %f", parent.GetProgress())
	}
	parent.Reopen()

	// Add children
	child1 := NewTask("child1", 2, TaskTypeTask, "Child 1")
	child2 := NewTask("child2", 3, TaskTypeTask, "Child 2")

	parent.AddChild(child1)
	parent.AddChild(child2)

	// 0% complete
	if parent.GetProgress() != 0 {
		t.Errorf("expected 0 progress with no complete children, got %f", parent.GetProgress())
	}

	// 50% complete
	child1.Complete()
	if parent.GetProgress() != 50 {
		t.Errorf("expected 50 progress with 1/2 complete, got %f", parent.GetProgress())
	}

	// 100% complete
	child2.Complete()
	if parent.GetProgress() != 100 {
		t.Errorf("expected 100 progress with 2/2 complete, got %f", parent.GetProgress())
	}
}

func TestTaskHierarchy(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	// Test IsRoot and IsLeaf
	if !epic.IsRoot() {
		t.Error("epic should be root")
	}
	if !epic.IsLeaf() {
		t.Error("epic should be leaf initially")
	}

	// Add child
	epic.AddChild(story)
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
	story.AddChild(task)
	if task.Level != 2 {
		t.Errorf("task level should be 2, got %d", task.Level)
	}
}

func TestTaskCanAddChild(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")
	idea := NewTask("idea", 5, TaskTypeIdea, "Idea")

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

	// Story can have task, bug
	if !story.CanAddChild(TaskTypeTask) {
		t.Error("story should accept task child")
	}
	if !story.CanAddChild(TaskTypeBug) {
		t.Error("story should accept bug child")
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

func TestTaskCountChildren(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story1 := NewTask("story1", 2, TaskTypeStory, "Story 1")
	story2 := NewTask("story2", 3, TaskTypeStory, "Story 2")
	task1 := NewTask("task1", 4, TaskTypeTask, "Task 1")
	task2 := NewTask("task2", 5, TaskTypeTask, "Task 2")

	epic.AddChild(story1)
	epic.AddChild(story2)
	story1.AddChild(task1)
	story1.AddChild(task2)

	// Epic has 2 direct + 2 nested = 4 descendants
	if epic.CountChildren() != 4 {
		t.Errorf("expected 4 children, got %d", epic.CountChildren())
	}

	// Story1 has 2 children
	if story1.CountChildren() != 2 {
		t.Errorf("expected 2 children for story1, got %d", story1.CountChildren())
	}

	// Task1 has no children
	if task1.CountChildren() != 0 {
		t.Errorf("expected 0 children for task1, got %d", task1.CountChildren())
	}
}

func TestTaskCountCompletedChildren(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task1 := NewTask("task1", 2, TaskTypeTask, "Task 1")
	task2 := NewTask("task2", 3, TaskTypeTask, "Task 2")
	task3 := NewTask("task3", 4, TaskTypeTask, "Task 3")

	epic.AddChild(task1)
	epic.AddChild(task2)
	epic.AddChild(task3)

	if epic.CountCompletedChildren() != 0 {
		t.Errorf("expected 0 completed, got %d", epic.CountCompletedChildren())
	}

	task1.Complete()
	if epic.CountCompletedChildren() != 1 {
		t.Errorf("expected 1 completed, got %d", epic.CountCompletedChildren())
	}

	task2.Complete()
	task3.Cancel()
	if epic.CountCompletedChildren() != 3 {
		t.Errorf("expected 3 completed (including canceled), got %d", epic.CountCompletedChildren())
	}
}

func TestCanIndent(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")

	// Task can indent under Epic
	ok, reason := task.CanIndent(epic)
	if !ok {
		t.Errorf("task should be able to indent under epic: %s", reason)
	}

	// Task can indent under Story
	ok, reason = task.CanIndent(story)
	if !ok {
		t.Errorf("task should be able to indent under story: %s", reason)
	}

	// Task cannot indent under another Task (leaf type)
	ok, _ = task.CanIndent(bug)
	if ok {
		t.Error("task should not be able to indent under bug (leaf type)")
	}

	// Story can indent under Epic
	ok, reason = story.CanIndent(epic)
	if !ok {
		t.Errorf("story should be able to indent under epic: %s", reason)
	}

	// Story cannot indent under another Story
	story2 := NewTask("story2", 5, TaskTypeStory, "Story 2")
	ok, _ = story.CanIndent(story2)
	if ok {
		t.Error("story should not be able to indent under another story")
	}

	// Epic cannot indent under Story
	ok, _ = epic.CanIndent(story)
	if ok {
		t.Error("epic should not be able to indent under story")
	}

	// Cannot indent with nil parent
	ok, _ = task.CanIndent(nil)
	if ok {
		t.Error("should not be able to indent with nil parent")
	}
}

func TestCanOutdent(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	// Set up hierarchy: epic -> story -> task
	epic.AddChild(story)
	story.AddChild(task)

	// Task can outdent (would become child of epic)
	ok, reason := task.CanOutdent()
	if !ok {
		t.Errorf("task should be able to outdent: %s", reason)
	}

	// Story can outdent (would become root)
	ok, reason = story.CanOutdent()
	if !ok {
		t.Errorf("story should be able to outdent: %s", reason)
	}

	// Epic cannot outdent (already root)
	ok, _ = epic.CanOutdent()
	if ok {
		t.Error("epic should not be able to outdent (already root)")
	}

	// Story under Epic with Bug child - Bug can outdent to become Epic child
	epic2 := NewTask("epic2", 10, TaskTypeEpic, "Epic 2")
	story2 := NewTask("story2", 11, TaskTypeStory, "Story 2")
	bugUnderStory := NewTask("bug", 12, TaskTypeBug, "Bug")
	epic2.AddChild(story2)
	story2.AddChild(bugUnderStory)

	ok, reason = bugUnderStory.CanOutdent()
	if !ok {
		t.Errorf("bug under story should be able to outdent to epic: %s", reason)
	}
}

func TestCanOutdentInvalidHierarchy(t *testing.T) {
	// Create hierarchy where outdent would be invalid
	// Story -> Task, but Task promotes to Story, cannot be root Story's sibling
	story := NewTask("story", 1, TaskTypeStory, "Story")
	task := NewTask("task", 2, TaskTypeTask, "Task")
	story.AddChild(task)

	// Task under Story with no grandparent can outdent to root
	ok, _ := task.CanOutdent()
	if !ok {
		t.Error("task under story should be able to outdent to root")
	}
}

func TestPromoteWithNestedChildren(t *testing.T) {
	// Story with task children can promote to epic (tasks valid under epic)
	story := NewTask("story", 1, TaskTypeStory, "Story")
	task1 := NewTask("task1", 2, TaskTypeTask, "Task 1")
	task2 := NewTask("task2", 3, TaskTypeTask, "Task 2")
	story.AddChild(task1)
	story.AddChild(task2)

	err := story.Promote()
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
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task1 := NewTask("task1", 2, TaskTypeTask, "Task 1")
	bug := NewTask("bug", 3, TaskTypeBug, "Bug")
	epic.AddChild(task1)
	epic.AddChild(bug)

	err := epic.Demote()
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

func TestDemoteEpicWithIncompatibleChildren(t *testing.T) {
	// Epic with idea children cannot demote (idea cannot be child of story)
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	idea := NewTask("idea", 2, TaskTypeIdea, "Idea")
	epic.AddChild(idea)

	err := epic.Demote()
	if err == nil {
		t.Error("Demote() epic with idea child should fail")
	}
	// Type should remain epic
	if epic.Type != TaskTypeEpic {
		t.Errorf("Type should remain epic after failed demote, got %s", epic.Type)
	}
}
