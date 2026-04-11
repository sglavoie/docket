package models

import "testing"

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
