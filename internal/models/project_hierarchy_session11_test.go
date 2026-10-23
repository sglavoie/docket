package models

import (
	"strings"
	"testing"
	"time"
)

func TestReparentTaskRejectsSubtreePastMaximumDepthWithoutMutation(t *testing.T) {
	project, err := NewProject("project", "Project", "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}

	chain := make([]*Task, MaxNestingDepth)
	for i := range chain {
		chain[i], err = NewGenericTask(string(rune('a'+i)), i+1, "Ancestor")
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			parentID := chain[i-1].ID
			chain[i].ParentID = &parentID
		}
	}
	moved, err := NewGenericTask("moved", 20, "Moved")
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewGenericTask("child", 21, "Child")
	if err != nil {
		t.Fatal(err)
	}
	child.ParentID = &moved.ID
	project.Tasks = append(chain, moved, child)
	project.BuildTaskTree()

	err = project.ReparentTaskAt(moved, chain[len(chain)-1], time.Now())
	if err == nil || !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("ReparentTask error = %v, want nesting-depth rejection", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("rejected reparent changed parent to %v", moved.ParentID)
	}
	if child.ParentID == nil || *child.ParentID != moved.ID {
		t.Fatalf("rejected reparent changed child parent to %v", child.ParentID)
	}
}

func TestReparentTaskRejectsParentOutsideProject(t *testing.T) {
	project, err := NewProject("project", "Project", "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("task", 1, "Task")
	if err != nil {
		t.Fatal(err)
	}
	external, err := NewGenericTask("external", 2, "External")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{task}

	if err := project.ReparentTaskAt(task, external, time.Now()); err == nil {
		t.Fatal("ReparentTask accepted a parent outside the project")
	}
	if task.ParentID != nil {
		t.Fatalf("rejected reparent changed parent to %v", task.ParentID)
	}
}

func TestSwapSiblingTasksRejectsDifferentParentGroups(t *testing.T) {
	project, err := NewProject("project", "Project", "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	parentA, _ := NewGenericTask("parent-a", 1, "Parent A")
	parentB, _ := NewGenericTask("parent-b", 2, "Parent B")
	childA, _ := NewGenericTask("child-a", 3, "Child A")
	childB, _ := NewGenericTask("child-b", 4, "Child B")
	childA.ParentID = &parentA.ID
	childB.ParentID = &parentB.ID
	project.Tasks = []*Task{parentA, childA, parentB, childB}

	if err := project.SwapSiblingTasksAt(childA, childB, time.Now()); err == nil {
		t.Fatal("SwapSiblingTasks accepted tasks with different parents")
	}
	assertTaskIDSequence(t, "project.Tasks", project.Tasks, []string{"parent-a", "child-a", "parent-b", "child-b"})
}
