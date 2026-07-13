package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// newReparentTask builds a task with an explicit ID so the reparent tests can
// resolve tasks across the wired snapshot by a stable identity.
func newReparentTask(t *testing.T, id string, shortID int, typ models.TaskType, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, typ, title)
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

	alphaEpicA := newReparentTask(t, "alpha-epic-a", 1, models.TaskTypeEpic, "Epic A")
	alphaEpicB := newReparentTask(t, "alpha-epic-b", 2, models.TaskTypeEpic, "Epic B")
	alphaLeaf := newReparentTask(t, "alpha-leaf", 3, models.TaskTypeTask, "Leaf under A")
	alphaLeaf.ParentID = strPtr("alpha-epic-a")
	betaEpic := newReparentTask(t, "beta-epic", 1, models.TaskTypeEpic, "Beta Epic")

	env := newKanbanBulkEnv(t, []*models.Task{alphaEpicA, alphaEpicB, alphaLeaf}, []*models.Task{betaEpic})
	m := newCrossOutlinerFromEnv(env)

	newParentID := "alpha-epic-b"
	_, cmd := m.handleCrossProjectReparent(ParentSelectedMsg{
		ReparentTaskID: "alpha-leaf",
		ParentID:       &newParentID,
		Source:         ViewCrossProjectOutliner,
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

// TestCrossProjectReparent_InvalidTargetNoSnapshot verifies that supplying an
// invalid parent (type-incompatible: task -> task) is rejected by the
// application layer before any commit, so the coordinator emits a
// TransactionErrorMsg and the on-disk parent is unchanged.
func TestCrossProjectReparent_InvalidTargetNoSnapshot(t *testing.T) {
	leafA := newReparentTask(t, "leaf-a", 1, models.TaskTypeTask, "Leaf A")
	leafB := newReparentTask(t, "leaf-b", 2, models.TaskTypeTask, "Leaf B")

	env := newKanbanBulkEnv(t, []*models.Task{leafA, leafB})
	m := newCrossOutlinerFromEnv(env)

	// leafA cannot be a child of leafB (task type can't parent another task).
	invalidParent := "leaf-b"
	_, cmd := m.handleCrossProjectReparent(ParentSelectedMsg{
		ReparentTaskID: "leaf-a",
		ParentID:       &invalidParent,
		Source:         ViewCrossProjectOutliner,
	})
	if cmd == nil {
		t.Fatal("expected a command from an invalid reparent")
	}
	if _, ok := cmd().(TransactionErrorMsg); !ok {
		t.Fatalf("expected TransactionErrorMsg for invalid reparent, got %T", cmd())
	}

	// The parent must remain unchanged on disk (no commit for a rejected change).
	got := env.reload(t, 0).GetTaskByID("leaf-a")
	if got == nil {
		t.Fatal("leaf-a missing after reload")
	}
	if got.ParentID != nil {
		t.Errorf("leaf-a ParentID should still be nil, got %q", *got.ParentID)
	}
}

// TestCrossProjectReparentToRoot verifies that picking "None" (nil ParentID)
// detaches the task from its current parent in cross-project mode.
func TestCrossProjectReparentToRoot(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	epic := newReparentTask(t, "epic", 1, models.TaskTypeEpic, "Epic")
	leaf := newReparentTask(t, "leaf", 2, models.TaskTypeTask, "Leaf")
	leaf.ParentID = strPtr("epic")

	env := newKanbanBulkEnv(t, []*models.Task{epic, leaf})
	m := newCrossOutlinerFromEnv(env)

	_, cmd := m.handleCrossProjectReparent(ParentSelectedMsg{
		ReparentTaskID: "leaf",
		ParentID:       nil,
		Source:         ViewCrossProjectOutliner,
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
