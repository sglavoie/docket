package models

import (
	"testing"
	"time"
)

func newHierarchyReconcileProject(t *testing.T) *Project {
	t.Helper()
	project, err := NewProject("test", "Test", ".")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func newHierarchyReconcileTask(t *testing.T, id string, shortID int, title string) *Task {
	t.Helper()
	task, err := NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestReconcileParentCompletionSingleReopen(t *testing.T) {
	project := newHierarchyReconcileProject(t)

	parent := newHierarchyReconcileTask(t, "parent", 1, "Parent")
	child := newHierarchyReconcileTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	parent.Complete()
	// Child left incomplete → parent is now in an illegal completed state.
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletionAt(time.Now())
	if len(reopened) != 1 {
		t.Errorf("expected 1 reopened, got %d", len(reopened))
	}
	if parent.State() != StateDoing {
		t.Errorf("expected parent reopened to in_progress, got %s", parent.State())
	}
	if parent.CompletedAt != nil {
		t.Error("expected parent CompletedAt cleared")
	}
}

func TestReconcileParentCompletionFixpointGrandparent(t *testing.T) {
	project := newHierarchyReconcileProject(t)

	epic := newHierarchyReconcileTask(t, "epic", 1, "Epic")
	story := newHierarchyReconcileTask(t, "story", 2, "Story")
	task := newHierarchyReconcileTask(t, "task", 3, "Task")

	if err := story.AddChild(task); err != nil {
		t.Fatal(err)
	}
	if err := epic.AddChild(story); err != nil {
		t.Fatal(err)
	}
	epic.Complete()
	story.Complete()
	// Leaf task left incomplete → both story and epic are illegal completions.
	project.Tasks = []*Task{epic, story, task}

	reopened := project.ReconcileParentCompletionAt(time.Now())
	if len(reopened) != 2 {
		t.Errorf("expected 2 reopened (story then epic), got %d", len(reopened))
	}
	if story.State() != StateDoing {
		t.Errorf("expected story reopened, got %s", story.State())
	}
	if epic.State() != StateDoing {
		t.Errorf("expected epic reopened via fixpoint, got %s", epic.State())
	}
}

func TestReconcileParentCompletionCanceledParent(t *testing.T) {
	project := newHierarchyReconcileProject(t)

	parent := newHierarchyReconcileTask(t, "parent", 1, "Parent")
	child := newHierarchyReconcileTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	parent.Cancel()
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletionAt(time.Now())
	if len(reopened) != 1 {
		t.Errorf("expected 1 reopened, got %d", len(reopened))
	}
	if parent.State() != StateDoing {
		t.Errorf("expected canceled parent reopened to in_progress, got %s", parent.State())
	}
}

func TestReconcileParentCompletionNoOpWhenConsistent(t *testing.T) {
	project := newHierarchyReconcileProject(t)

	parent := newHierarchyReconcileTask(t, "parent", 1, "Parent")
	child := newHierarchyReconcileTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	child.Complete()
	parent.Complete()
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletionAt(time.Now())
	if len(reopened) != 0 {
		t.Errorf("expected 0 reopened for consistent tree, got %d", len(reopened))
	}
	if parent.State() != StateDone {
		t.Errorf("expected parent to stay done, got %s", parent.State())
	}
	if child.State() != StateDone {
		t.Errorf("expected child to stay done, got %s", child.State())
	}
}
