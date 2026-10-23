package models

import (
	"testing"
	"time"
)

// TestProjectMutatorsWriteTheSuppliedClock holds the structural mutators to the
// caller's clock: each one stamps the instant it was handed, never the wall
// clock, so a service with an injected clock writes the same touch time and
// state clock its own judgement used.
func TestProjectMutatorsWriteTheSuppliedClock(t *testing.T) {
	now := time.Date(2031, time.February, 3, 4, 5, 6, 0, time.UTC)

	t.Run("reconciling a parent reopens it at now", func(t *testing.T) {
		project := newHierarchyReconcileProject(t)
		parent := newHierarchyReconcileTask(t, "parent", 1, "Parent")
		child := newHierarchyReconcileTask(t, "child", 2, "Child")
		if err := parent.AddChild(child); err != nil {
			t.Fatal(err)
		}
		parent.Complete()
		project.Tasks = []*Task{parent, child}

		if reopened := project.ReconcileParentCompletionAt(now); len(reopened) != 1 {
			t.Fatalf("reopened = %v, want the parent", reopened)
		}
		if parent.State() != StateDoing || !parent.StateChangedAt.Equal(now) || !parent.UpdatedAt.Equal(now) {
			t.Fatalf("reopened parent = state %s clock %s touched %s, want doing at %s",
				parent.State(), parent.StateChangedAt, parent.UpdatedAt, now)
		}
	})

	t.Run("a parent assignment and a sibling move touch at now", func(t *testing.T) {
		project := newHierarchyReconcileProject(t)
		first := newHierarchyReconcileTask(t, "first", 1, "First")
		second := newHierarchyReconcileTask(t, "second", 2, "Second")
		third := newHierarchyReconcileTask(t, "third", 3, "Third")
		project.Tasks = []*Task{first, second, third}
		project.BuildTaskTree()

		if err := project.MoveTaskUpAt(second, now); err != nil {
			t.Fatalf("MoveTaskUpAt: %v", err)
		}
		if !first.UpdatedAt.Equal(now) || !second.UpdatedAt.Equal(now) {
			t.Fatalf("swap touched %s and %s, want %s", first.UpdatedAt, second.UpdatedAt, now)
		}

		later := now.Add(time.Hour)
		if err := project.ReparentTaskAt(third, first, later); err != nil {
			t.Fatalf("ReparentTaskAt: %v", err)
		}
		if !third.UpdatedAt.Equal(later) {
			t.Fatalf("reparent touched %s, want %s", third.UpdatedAt, later)
		}

		latest := later.Add(time.Hour)
		if err := project.ReorderSiblingTasksAt([]*Task{first, second}, latest); err != nil {
			t.Fatalf("ReorderSiblingTasksAt: %v", err)
		}
		if !first.UpdatedAt.Equal(latest) || !second.UpdatedAt.Equal(latest) {
			t.Fatalf("reorder touched %s and %s, want %s", first.UpdatedAt, second.UpdatedAt, latest)
		}
	})
}

// TestAttributeMutatorsWriteTheSuppliedClock extends that rule to the attribute
// and catalog mutators a service reaches: pin, tags, attachment and comment
// removal, archive, cross-project move, and project creation.
func TestAttributeMutatorsWriteTheSuppliedClock(t *testing.T) {
	now := time.Date(2031, time.February, 3, 4, 5, 6, 0, time.UTC)
	task := newHierarchyReconcileTask(t, "task", 1, "Task")
	task.Attachments = []Attachment{{Filename: "a.txt"}}
	task.AddCommentAt("note", now.Add(-time.Hour))

	steps := []struct {
		name string
		run  func(time.Time)
	}{
		{"PinAt", task.PinAt},
		{"UnpinAt", task.UnpinAt},
		{"AddTagAt", func(at time.Time) { task.AddTagAt("garden", at) }},
		{"RemoveTagAt", func(at time.Time) { task.RemoveTagAt("garden", at) }},
		{"RemoveAttachmentAt", func(at time.Time) { task.RemoveAttachmentAt(0, at) }},
		{"RemoveCommentAt", func(at time.Time) { task.RemoveCommentAt(0, at) }},
	}
	for i, step := range steps {
		at := now.Add(time.Duration(i) * time.Minute)
		step.run(at)
		if !task.UpdatedAt.Equal(at) {
			t.Fatalf("%s touched %s, want %s", step.name, task.UpdatedAt, at)
		}
	}

	project, err := NewProjectAt("p1", "Garden", "projects/garden", now)
	if err != nil || !project.CreatedAt.Equal(now) || !project.UpdatedAt.Equal(now) {
		t.Fatalf("NewProjectAt = %+v, %v, want stamped %s", project, err, now)
	}
	later := now.Add(time.Hour)
	project.ArchiveAt(later)
	if project.ArchivedAt == nil || !project.ArchivedAt.Equal(later) || !project.UpdatedAt.Equal(later) {
		t.Fatalf("ArchiveAt left archived %v touched %s, want %s", project.ArchivedAt, project.UpdatedAt, later)
	}
	latest := later.Add(time.Hour)
	project.UnarchiveAt(latest)
	if !project.UpdatedAt.Equal(latest) {
		t.Fatalf("UnarchiveAt touched %s, want %s", project.UpdatedAt, latest)
	}
	project.PinAt(later)
	if project.PinnedAt == nil || !project.PinnedAt.Equal(later) || !project.UpdatedAt.Equal(later) {
		t.Fatalf("project PinAt left pinned %v touched %s, want %s", project.PinnedAt, project.UpdatedAt, later)
	}
	project.UnpinAt(latest)
	if project.Pinned || !project.UpdatedAt.Equal(latest) {
		t.Fatalf("project UnpinAt left pinned %v touched %s, want %s", project.Pinned, project.UpdatedAt, latest)
	}

	source := newHierarchyReconcileProject(t)
	moved := newHierarchyReconcileTask(t, "moved", 1, "Moved")
	source.Tasks = []*Task{moved}
	if _, err := source.MoveTaskToProjectAt(moved, project, latest); err != nil {
		t.Fatalf("MoveTaskToProjectAt: %v", err)
	}
	if !moved.UpdatedAt.Equal(latest) {
		t.Fatalf("cross-project move touched %s, want %s", moved.UpdatedAt, latest)
	}
}
