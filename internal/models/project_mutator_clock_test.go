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
