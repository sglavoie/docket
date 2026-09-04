package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// newReparentTask builds a task with an explicit ID so the reparent tests can
// resolve tasks across the wired snapshot by a stable identity.
func newReparentTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask %s: %v", id, err)
	}
	return task
}

// TestCrossProjectReparent verifies that a ParentSelectedMsg received by the
// OutlinerModel in cross-project mode reparents the task in its owning project
// (not some other loaded project) through the single-task coordinator, commits
// one ledger-backed transaction, and that the ledger undo restores it.
func TestCrossProjectReparent(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	alphaEpicA := newReparentTask(t, "alpha-epic-a", 1, "Epic A")
	alphaEpicB := newReparentTask(t, "alpha-epic-b", 2, "Epic B")
	alphaLeaf := newReparentTask(t, "alpha-leaf", 3, "Leaf under A")
	alphaLeaf.ParentID = strPtr("alpha-epic-a")
	betaEpic := newReparentTask(t, "beta-epic", 1, "Beta Epic")

	env := newCrossProjectTestEnv(t, []*models.Task{alphaEpicA, alphaEpicB, alphaLeaf}, []*models.Task{betaEpic})
	m := newCrossOutlinerFromEnv(env)

	newParentID := "alpha-epic-b"
	_, cmd := m.handleCrossProjectReparent(listsurface.ParentSelectedMsg{
		ReparentTask: taskref.TaskIdentity{ProjectID: env.projects[0].ID, TaskID: "alpha-leaf"},
		ParentID:     &newParentID,
		Source:       presentation.ViewCrossProjectOutliner,
	})
	runResult(t, cmd)

	leaf := env.reload(t, 0).GetTaskByID("alpha-leaf")
	if leaf == nil {
		t.Fatal("leaf not found after reload")
	}
	if leaf.ParentID == nil || *leaf.ParentID != "alpha-epic-b" {
		got := "nil"
		if leaf.ParentID != nil {
			got = *leaf.ParentID
		}
		t.Errorf("leaf parent: want %q, got %q", "alpha-epic-b", got)
	}

	// The ledger-primary undo restores the pre-mutation parent.
	env.undo(t)
	leaf = env.reload(t, 0).GetTaskByID("alpha-leaf")
	if leaf == nil || leaf.ParentID == nil || *leaf.ParentID != "alpha-epic-a" {
		got := "nil"
		if leaf != nil && leaf.ParentID != nil {
			got = *leaf.ParentID
		}
		t.Errorf("leaf parent after undo: want %q, got %q", "alpha-epic-a", got)
	}
}

// TestCrossProjectReparent_GenericTaskParent verifies that generic tasks can be
// nested without the retired type restrictions.
func TestCrossProjectReparent_GenericTaskParent(t *testing.T) {
	leafA := newReparentTask(t, "leaf-a", 1, "Leaf A")
	leafB := newReparentTask(t, "leaf-b", 2, "Leaf B")

	env := newCrossProjectTestEnv(t, []*models.Task{leafA, leafB})
	m := newCrossOutlinerFromEnv(env)

	parentID := "leaf-b"
	_, cmd := m.handleCrossProjectReparent(listsurface.ParentSelectedMsg{
		ReparentTask: taskref.TaskIdentity{ProjectID: env.projects[0].ID, TaskID: "leaf-a"},
		ParentID:     &parentID,
		Source:       presentation.ViewCrossProjectOutliner,
	})
	runResult(t, cmd)

	got := env.reload(t, 0).GetTaskByID("leaf-a")
	if got == nil {
		t.Fatal("leaf-a missing after reload")
	}
	if got.ParentID == nil || *got.ParentID != parentID {
		t.Errorf("leaf-a ParentID = %v, want %q", got.ParentID, parentID)
	}
}

// TestCrossProjectReparentToRoot verifies that picking "None" (nil ParentID)
// detaches the task from its current parent in cross-project mode.
func TestCrossProjectReparentToRoot(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	epic := newReparentTask(t, "epic", 1, "Epic")
	leaf := newReparentTask(t, "leaf", 2, "Leaf")
	leaf.ParentID = strPtr("epic")

	env := newCrossProjectTestEnv(t, []*models.Task{epic, leaf})
	m := newCrossOutlinerFromEnv(env)

	_, cmd := m.handleCrossProjectReparent(listsurface.ParentSelectedMsg{
		ReparentTask: taskref.TaskIdentity{ProjectID: env.projects[0].ID, TaskID: "leaf"},
		ParentID:     nil,
		Source:       presentation.ViewCrossProjectOutliner,
	})
	runResult(t, cmd)

	got := env.reload(t, 0).GetTaskByID("leaf")
	if got == nil {
		t.Fatal("leaf missing")
	}
	if got.ParentID != nil {
		t.Errorf("leaf ParentID: want nil, got %q", *got.ParentID)
	}
}
