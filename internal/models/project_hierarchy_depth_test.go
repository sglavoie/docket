package models

import (
	"fmt"
	"testing"
)

func TestBuildTaskTree_MaxNestingDepth(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create a chain of 8 tasks nested sequentially: depth 0 -> 1 -> ... -> 7
	const chainLen = 8
	tasks := make([]*Task, chainLen)
	for i := range chainLen {
		task, err := NewGenericTask(fmt.Sprintf("task-%d", i), i+1, fmt.Sprintf("Task depth %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			parentID := tasks[i-1].ID
			task.ParentID = &parentID
		}
		tasks[i] = task
	}

	project.Tasks = tasks
	project.BuildTaskTree()

	// Tasks at depth 0 through MaxNestingDepth (6) should keep their hierarchy
	for i := 0; i <= MaxNestingDepth; i++ {
		if tasks[i].Level != i {
			t.Errorf("task at intended depth %d: got Level=%d, want %d", i, tasks[i].Level, i)
		}
		if i == 0 {
			if tasks[i].ParentID != nil {
				t.Errorf("root task should have nil ParentID")
			}
		} else {
			if tasks[i].ParentID == nil || *tasks[i].ParentID != tasks[i-1].ID {
				t.Errorf("task at depth %d should still have parent %s", i, tasks[i-1].ID)
			}
		}
	}

	// Tasks beyond MaxNestingDepth should be detached to root
	for i := MaxNestingDepth + 1; i < chainLen; i++ {
		if tasks[i].Level != 0 {
			t.Errorf("task at intended depth %d: got Level=%d, want 0 (should be detached)", i, tasks[i].Level)
		}
		if tasks[i].ParentID != nil {
			t.Errorf("task at intended depth %d: ParentID should be nil after detach, got %s", i, *tasks[i].ParentID)
		}
	}
}

func TestBuildTaskTree_MaxNestingDepthRelevelSubtree(t *testing.T) {
	// Regression: when a deeply nested task with children is detached to root,
	// its children must be re-leveled relative to the new root (level 0),
	// not retain their original (stale) levels.
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create chain: 0 -> 1 -> ... -> MaxNestingDepth+1 -> child_a, child_b
	chainLen := MaxNestingDepth + 2 // one past the cap
	tasks := make([]*Task, chainLen)
	for i := range chainLen {
		var task *Task
		task, err = NewGenericTask(fmt.Sprintf("chain-%d", i), i+1, fmt.Sprintf("Chain %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			parentID := tasks[i-1].ID
			task.ParentID = &parentID
		}
		tasks[i] = task
	}

	// Add two children to the task that will be detached
	detachedRoot := tasks[chainLen-1]
	childA, err := NewGenericTask("child-a", 100, "Child A")
	if err != nil {
		t.Fatal(err)
	}
	childB, err := NewGenericTask("child-b", 101, "Child B")
	if err != nil {
		t.Fatal(err)
	}
	detachedID := detachedRoot.ID
	childA.ParentID = &detachedID
	childB.ParentID = &detachedID

	project.Tasks = append(tasks, childA, childB)
	project.BuildTaskTree()

	// The detached root should be at level 0
	if detachedRoot.Level != 0 {
		t.Errorf("detached root: got Level=%d, want 0", detachedRoot.Level)
	}
	// Its children should be at level 1 (not the original deep level)
	if childA.Level != 1 {
		t.Errorf("child A of detached root: got Level=%d, want 1", childA.Level)
	}
	if childB.Level != 1 {
		t.Errorf("child B of detached root: got Level=%d, want 1", childB.Level)
	}
}

func TestAddChild_ExceedsMaxNestingDepth(t *testing.T) {
	parent, err := NewGenericTask("parent", 1, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	parent.Level = MaxNestingDepth // simulate being at the deepest allowed level

	child, err := NewGenericTask("child", 2, "Child")
	if err != nil {
		t.Fatal(err)
	}

	err = parent.AddChild(child)
	if err == nil {
		t.Error("expected error when adding child would exceed MaxNestingDepth")
	}
}
