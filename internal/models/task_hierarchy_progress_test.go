package models

import "testing"

func TestTaskProgress(t *testing.T) {
	parent, err := NewGenericTask("parent", 1, "Parent Epic")
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
	if err = parent.ApplyStateTransition(StateInbox); err != nil {
		t.Fatal(err)
	}

	// Add children
	child1, err := NewGenericTask("child1", 2, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewGenericTask("child2", 3, "Child 2")
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

func TestTaskCountChildren(t *testing.T) {
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story1, err := NewGenericTask("story1", 2, "Story 1")
	if err != nil {
		t.Fatal(err)
	}
	story2, err := NewGenericTask("story2", 3, "Story 2")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewGenericTask("task1", 4, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task2", 5, "Task 2")
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
	epic, err := NewGenericTask("epic", 1, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewGenericTask("task1", 2, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("task2", 3, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewGenericTask("task3", 4, "Task 3")
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

func TestMaxDescendantDepth(t *testing.T) {
	root, _ := NewGenericTask("root", 1, "Root")
	root.Level = 0
	child, _ := NewGenericTask("child", 2, "Child")
	child.Level = 1
	grandchild, _ := NewGenericTask("gc", 3, "Grandchild")
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
