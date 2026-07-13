package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestGetVisibleSiblings_FollowsNodeOrder pins the regression at the heart
// of the J/K reorder bug: getVisibleSiblings must source its order from
// m.nodes (display order after sortNodes), not m.project.GetSiblings
// (model order from p.Tasks). Without this, J/K swaps target the wrong
// neighbor in sorted/active-prioritized views.
func TestGetVisibleSiblings_FollowsNodeOrder(t *testing.T) {
	t1 := &models.Task{ID: "t1", Title: "T1", Type: models.TaskTypeTask, WorkflowState: models.StateReady}
	t2 := &models.Task{ID: "t2", Title: "T2", Type: models.TaskTypeTask, WorkflowState: models.StateReady}
	t3 := &models.Task{ID: "t3", Title: "T3", Type: models.TaskTypeTask, WorkflowState: models.StateReady}

	// p.Tasks order is t1, t2, t3 — the "model" order.
	project := &models.Project{Tasks: []*models.Task{t1, t2, t3}}

	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeAll,
	}
	// Build m.nodes in a deliberately different (post-sortNodes) order: t3, t1, t2.
	m.nodes = []*TreeNode{
		{Task: t3, Visible: true},
		{Task: t1, Visible: true},
		{Task: t2, Visible: true},
	}

	got := m.getVisibleSiblings(t1)
	wantIDs := []string{"t3", "t1", "t2"}
	if len(got) != len(wantIDs) {
		t.Fatalf("len(siblings) = %d, want %d (got %+v)", len(got), len(wantIDs), got)
	}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Errorf("siblings[%d].ID = %q, want %q (display order required)", i, got[i].ID, id)
		}
	}
}

// TestGetVisibleSiblings_ExcludesDifferentParent verifies that the helper
// only returns siblings sharing the same parent — both nil-parent (root)
// and same-ParentID matches are accepted, while differing parents are
// filtered out.
func TestGetVisibleSiblings_ExcludesDifferentParent(t *testing.T) {
	parentA := &models.Task{ID: "pa", Title: "PA", Type: models.TaskTypeEpic, WorkflowState: models.StateReady}
	parentB := &models.Task{ID: "pb", Title: "PB", Type: models.TaskTypeEpic, WorkflowState: models.StateReady}

	childA1 := &models.Task{ID: "a1", Title: "A1", Type: models.TaskTypeTask, WorkflowState: models.StateReady, ParentID: &parentA.ID, Parent: parentA}
	childA2 := &models.Task{ID: "a2", Title: "A2", Type: models.TaskTypeTask, WorkflowState: models.StateReady, ParentID: &parentA.ID, Parent: parentA}
	childB1 := &models.Task{ID: "b1", Title: "B1", Type: models.TaskTypeTask, WorkflowState: models.StateReady, ParentID: &parentB.ID, Parent: parentB}

	parentA.Children = []*models.Task{childA1, childA2}
	parentB.Children = []*models.Task{childB1}

	project := &models.Project{
		Tasks: []*models.Task{parentA, parentB, childA1, childA2, childB1},
	}

	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeAll,
	}
	// Children of parentA and parentB intermixed in display order.
	m.nodes = []*TreeNode{
		{Task: childA1, Visible: true},
		{Task: childB1, Visible: true}, // different parent — must be excluded
		{Task: childA2, Visible: true},
	}

	got := m.getVisibleSiblings(childA1)
	if len(got) != 2 {
		t.Fatalf("expected 2 siblings (parentA children only), got %d: %+v", len(got), got)
	}
	if got[0].ID != "a1" || got[1].ID != "a2" {
		t.Errorf("got [%s, %s], want [a1, a2] in node order", got[0].ID, got[1].ID)
	}
}

// TestGetVisibleSiblings_AppliesPassesFilters verifies the helper respects
// passesFilters — e.g. completed tasks are filtered out in ViewModeAll.
func TestGetVisibleSiblings_AppliesPassesFilters(t *testing.T) {
	t1 := &models.Task{ID: "t1", Title: "T1", Type: models.TaskTypeTask, WorkflowState: models.StateReady}
	t2 := &models.Task{ID: "t2", Title: "T2", Type: models.TaskTypeTask, WorkflowState: models.StateDone} // excluded in non-Completed views
	t3 := &models.Task{ID: "t3", Title: "T3", Type: models.TaskTypeTask, WorkflowState: models.StateReady}

	project := &models.Project{Tasks: []*models.Task{t1, t2, t3}}
	m := &ProjectTreeModel{
		project:  project,
		viewMode: ViewModeAll,
	}
	m.nodes = []*TreeNode{
		{Task: t1, Visible: true},
		{Task: t2, Visible: true},
		{Task: t3, Visible: true},
	}

	got := m.getVisibleSiblings(t1)
	if len(got) != 2 {
		t.Fatalf("expected 2 siblings (t2 filtered out), got %d", len(got))
	}
	if got[0].ID != "t1" || got[1].ID != "t3" {
		t.Errorf("got [%s, %s], want [t1, t3]", got[0].ID, got[1].ID)
	}
}
