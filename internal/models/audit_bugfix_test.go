package models

import "testing"

// TestAddChildRejectsSubtreeExceedingDepth verifies that AddChild refuses to
// attach a child whose descendants would exceed MaxNestingDepth.
func TestAddChildRejectsSubtreeExceedingDepth(t *testing.T) {
	storyTop, err := NewGenericTask("s1", 1, "story top")
	if err != nil {
		t.Fatalf("NewTask story: %v", err)
	}
	childTask, err := NewGenericTask("c1", 2, "child task")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	if err = storyTop.AddChild(childTask); err != nil {
		t.Fatalf("AddChild child: %v", err)
	}

	target, err := NewGenericTask("p", 99, "deep epic")
	if err != nil {
		t.Fatalf("NewTask target: %v", err)
	}
	target.Level = MaxNestingDepth - 1
	if err = target.AddChild(storyTop); err == nil {
		t.Fatal("expected AddChild to reject subtree that exceeds MaxNestingDepth")
	}
}

func TestAddChildAllowsLeafAtMaxDepth(t *testing.T) {
	parent, err := NewGenericTask("p", 1, "parent")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parent.Level = MaxNestingDepth - 1
	leaf, err := NewGenericTask("l", 2, "leaf")
	if err != nil {
		t.Fatalf("NewTask leaf: %v", err)
	}
	if err := parent.AddChild(leaf); err != nil {
		t.Errorf("expected leaf child at depth %d to be allowed, got %v", MaxNestingDepth, err)
	}
}
