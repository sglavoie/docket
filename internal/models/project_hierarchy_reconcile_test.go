package models

import "testing"

func TestReconcileParentCompletionSingleReopen(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	parent := newDependencyTestTask(t, "parent", 1, "Parent")
	parent.Type = TaskTypeStory
	child := newDependencyTestTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	parent.Complete()
	// Child left incomplete → parent is now in an illegal completed state.
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletion()
	if len(reopened) != 1 {
		t.Errorf("expected 1 reopened, got %d", len(reopened))
	}
	if parent.Status != StatusInProgress {
		t.Errorf("expected parent reopened to in_progress, got %s", parent.Status)
	}
	if parent.CompletedAt != nil {
		t.Error("expected parent CompletedAt cleared")
	}
}

func TestReconcileParentCompletionFixpointGrandparent(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	epic := newDependencyTestTask(t, "epic", 1, "Epic")
	epic.Type = TaskTypeEpic
	story := newDependencyTestTask(t, "story", 2, "Story")
	story.Type = TaskTypeStory
	task := newDependencyTestTask(t, "task", 3, "Task")

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

	reopened := project.ReconcileParentCompletion()
	if len(reopened) != 2 {
		t.Errorf("expected 2 reopened (story then epic), got %d", len(reopened))
	}
	if story.Status != StatusInProgress {
		t.Errorf("expected story reopened, got %s", story.Status)
	}
	if epic.Status != StatusInProgress {
		t.Errorf("expected epic reopened via fixpoint, got %s", epic.Status)
	}
}

func TestReconcileParentCompletionCanceledParent(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	parent := newDependencyTestTask(t, "parent", 1, "Parent")
	parent.Type = TaskTypeStory
	child := newDependencyTestTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	parent.Cancel()
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletion()
	if len(reopened) != 1 {
		t.Errorf("expected 1 reopened, got %d", len(reopened))
	}
	if parent.Status != StatusInProgress {
		t.Errorf("expected canceled parent reopened to in_progress, got %s", parent.Status)
	}
}

func TestReconcileParentCompletionNoOpWhenConsistent(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	parent := newDependencyTestTask(t, "parent", 1, "Parent")
	parent.Type = TaskTypeStory
	child := newDependencyTestTask(t, "child", 2, "Child")
	if err := parent.AddChild(child); err != nil {
		t.Fatal(err)
	}
	child.Complete()
	parent.Complete()
	project.Tasks = []*Task{parent, child}

	reopened := project.ReconcileParentCompletion()
	if len(reopened) != 0 {
		t.Errorf("expected 0 reopened for consistent tree, got %d", len(reopened))
	}
	if parent.Status != StatusDone {
		t.Errorf("expected parent to stay done, got %s", parent.Status)
	}
	if child.Status != StatusDone {
		t.Errorf("expected child to stay done, got %s", child.Status)
	}
}
