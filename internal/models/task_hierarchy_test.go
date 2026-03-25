package models

import (
	"testing"
)

func TestTaskProgress(t *testing.T) {
	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}

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
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}

	if err := parent.AddChild(child1); err != nil {
		t.Fatal(err)
	}
	if err := parent.AddChild(child2); err != nil {
		t.Fatal(err)
	}

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

func TestTaskCountChildren(t *testing.T) {
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story1, err := NewTask("story1", 2, TaskTypeStory, "Story 1")
	if err != nil {
		t.Fatal(err)
	}
	story2, err := NewTask("story2", 3, TaskTypeStory, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewTask("task1", 4, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 5, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	if err := epic.AddChild(story1); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(story2); err != nil {
		t.Fatal(err)
	}
	if err := story1.AddChild(task1); err != nil {
		t.Fatal(err)
	}
	if err := story1.AddChild(task2); err != nil {
		t.Fatal(err)
	}

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
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
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
	task3, err := NewTask("task3", 4, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	if err := epic.AddChild(task1); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(task2); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(task3); err != nil {
		t.Fatal(err)
	}

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
	story2, err := NewTask("story2", 5, TaskTypeStory, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
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

	// Set up hierarchy: epic -> story -> task
	if err = epic.AddChild(story); err != nil {
		t.Fatal(err)
	}
	if err = story.AddChild(task); err != nil {
		t.Fatal(err)
	}

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
	epic2, err := NewTask("epic2", 10, TaskTypeEpic, "Epic 2")
	if err != nil {
		t.Fatal(err)
	}
	story2, err := NewTask("story2", 11, TaskTypeStory, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
	bugUnderStory, err := NewTask("bug", 12, TaskTypeBug, "Bug")
	if err != nil {
		t.Fatal(err)
	}
	if err := epic2.AddChild(story2); err != nil {
		t.Fatal(err)
	}
	if err := story2.AddChild(bugUnderStory); err != nil {
		t.Fatal(err)
	}

	ok, reason = bugUnderStory.CanOutdent()
	if !ok {
		t.Errorf("bug under story should be able to outdent to epic: %s", reason)
	}
}

func TestCanOutdentInvalidHierarchy(t *testing.T) {
	// Create hierarchy where outdent would be invalid
	// Story -> Task, but Task promotes to Story, cannot be root Story's sibling
	story, err := NewTask("story", 1, TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("task", 2, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	if err := story.AddChild(task); err != nil {
		t.Fatal(err)
	}

	// Task under Story with no grandparent can outdent to root
	ok, _ := task.CanOutdent()
	if !ok {
		t.Error("task under story should be able to outdent to root")
	}
}

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

func TestCanIndent_NestingDepthExceeded(t *testing.T) {
	// Build a chain of tasks right at MaxNestingDepth:
	// epic (level 0) -> story (level 1) -> ... -> task (level MaxNestingDepth)
	epic, err := NewTask("root", 1, TaskTypeEpic, "Root Epic")
	if err != nil {
		t.Fatal(err)
	}
	epic.Level = 0

	// Create a story at level MaxNestingDepth (simulating a deep chain)
	deepStory, err := NewTask("deep", 2, TaskTypeStory, "Deep Story")
	if err != nil {
		t.Fatal(err)
	}
	deepStory.Level = MaxNestingDepth

	// A leaf task trying to indent under a task at MaxNestingDepth should fail
	leaf, err := NewTask("leaf", 3, TaskTypeTask, "Leaf Task")
	if err != nil {
		t.Fatal(err)
	}
	leaf.Level = 0

	ok, reason := leaf.CanIndent(deepStory)
	if ok {
		t.Errorf("CanIndent should fail when target parent is at MaxNestingDepth, got ok=true")
	}
	if reason == "" {
		t.Error("expected a reason string for the failure")
	}
}

func TestCanIndent_SubtreeExceedsDepth(t *testing.T) {
	// Target parent is at level MaxNestingDepth-1.
	// Task to indent has a child (subtree depth = 1).
	// Indenting would put the task at MaxNestingDepth and its child at MaxNestingDepth+1.
	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	parent.Level = MaxNestingDepth - 1

	task, err := NewTask("task", 2, TaskTypeStory, "Story With Child")
	if err != nil {
		t.Fatal(err)
	}
	task.Level = 0

	child, err := NewTask("child", 3, TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatal(err)
	}
	child.Level = 1
	task.Children = []*Task{child}

	ok, reason := task.CanIndent(parent)
	if ok {
		t.Errorf("CanIndent should fail when subtree would exceed MaxNestingDepth, got ok=true")
	}
	if reason == "" {
		t.Error("expected a reason string for the failure")
	}
}

func TestCanIndent_NestingDepthAllowed(t *testing.T) {
	// Target parent is at level MaxNestingDepth-1, task has no children.
	// This should succeed since the task would be at MaxNestingDepth (the limit).
	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	parent.Level = MaxNestingDepth - 1

	task, err := NewTask("task", 2, TaskTypeTask, "Leaf Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Level = 0

	ok, reason := task.CanIndent(parent)
	if !ok {
		t.Errorf("CanIndent should succeed for leaf task under parent at MaxNestingDepth-1, got reason: %s", reason)
	}
}

func TestMaxDescendantDepth(t *testing.T) {
	root, _ := NewTask("root", 1, TaskTypeEpic, "Root")
	root.Level = 0
	child, _ := NewTask("child", 2, TaskTypeStory, "Child")
	child.Level = 1
	grandchild, _ := NewTask("gc", 3, TaskTypeTask, "Grandchild")
	grandchild.Level = 2
	root.Children = []*Task{child}
	child.Children = []*Task{grandchild}

	got := maxDescendantDepth(root)
	if got != 2 {
		t.Errorf("maxDescendantDepth = %d, want 2", got)
	}

	// Leaf node: depth == own level
	got = maxDescendantDepth(grandchild)
	if got != 2 {
		t.Errorf("maxDescendantDepth(leaf) = %d, want 2", got)
	}
}
