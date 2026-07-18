package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// setupFoldBoard builds a Today board with a single planned parent that has two
// levels of descendants (parent → child → grandchild), fully expanded.
func setupFoldBoard(t *testing.T) *FocusBoardModel {
	t.Helper()
	project, _ := models.NewProject("proj", "proj", "/tmp/test-proj")

	parent := makeBoardTask(t, "id-parent", 1, "Parent")
	child := makeBoardTask(t, "id-child", 2, "Child")
	grandchild := makeBoardTask(t, "id-grand", 3, "Grandchild")

	child.Children = []*models.Task{grandchild}
	parent.Children = []*models.Task{child}

	m := newTestFocusBoard()
	m.tabs.Active = boardTabToday
	m.childDepthLimit = -1
	m.dailyPlanIDs = map[string]bool{parent.ID: true}
	m.todayRows = []boardRow{
		{Task: parent, Project: project, Source: project.Name},
	}
	m.expandTodayChildren()
	m.applyFilter()

	// Sanity: fully expanded shows all three levels.
	if len(m.filtered) != 3 {
		t.Fatalf("setup: expected 3 rows fully expanded, got %d", len(m.filtered))
	}
	return m
}

// rootRowCount counts filtered rows that are tree roots (no parent).
func rootRowCount(m *FocusBoardModel) int {
	n := 0
	for _, r := range m.filtered {
		if r.ParentID == "" {
			n++
		}
	}
	return n
}

// TestFocusBoardCollapseToDepth verifies f 1/2/3 map to childDepthLimit 0/1/2 and
// rebuild the Today tree to the corresponding depth.
func TestFocusBoardCollapseToDepth(t *testing.T) {
	// f 1 → limit 0: only root rows remain.
	m := setupFoldBoard(t)
	m.focusBoardCollapseToDepthAction(0)()
	if m.childDepthLimit != 0 {
		t.Errorf("f 1: expected childDepthLimit 0, got %d", m.childDepthLimit)
	}
	if len(m.filtered) != 1 {
		t.Fatalf("f 1: expected 1 row, got %d", len(m.filtered))
	}
	if rootRowCount(m) != len(m.filtered) {
		t.Errorf("f 1: expected only root rows, got %d roots of %d", rootRowCount(m), len(m.filtered))
	}

	// f 2 → limit 1: exactly one child level below the root.
	m = setupFoldBoard(t)
	m.focusBoardCollapseToDepthAction(1)()
	if m.childDepthLimit != 1 {
		t.Errorf("f 2: expected childDepthLimit 1, got %d", m.childDepthLimit)
	}
	if len(m.filtered) != 2 {
		t.Fatalf("f 2: expected 2 rows, got %d", len(m.filtered))
	}

	// f 3 → limit 2: two child levels below the root.
	m = setupFoldBoard(t)
	m.focusBoardCollapseToDepthAction(2)()
	if m.childDepthLimit != 2 {
		t.Errorf("f 3: expected childDepthLimit 2, got %d", m.childDepthLimit)
	}
	if len(m.filtered) != 3 {
		t.Fatalf("f 3: expected 3 rows, got %d", len(m.filtered))
	}
}

// TestFocusBoardCollapseToDepthNoopOffToday verifies the depth-fold actions do
// nothing when the active tab is not Today.
func TestFocusBoardCollapseToDepthNoopOffToday(t *testing.T) {
	m := setupFoldBoard(t)
	m.tabs.Active = boardTabNext

	m.focusBoardCollapseToDepthAction(0)()
	if m.childDepthLimit != -1 {
		t.Errorf("off-Today: expected childDepthLimit unchanged (-1), got %d", m.childDepthLimit)
	}
}

// TestFocusBoardToggleFold verifies f a collapses the subtree under the cursor,
// a second f a restores it, and the cursor stays on the toggled parent.
func TestFocusBoardToggleFold(t *testing.T) {
	m := setupFoldBoard(t)
	m.cursor = 0 // parent row

	// First f a: collapse the parent's subtree.
	m.focusBoardToggleFold()
	if len(m.filtered) != 1 {
		t.Fatalf("f a collapse: expected 1 row, got %d", len(m.filtered))
	}
	if m.filtered[0].Task.ID != "id-parent" {
		t.Errorf("f a collapse: cursor row should be parent, got %q", m.filtered[0].Task.ID)
	}
	if m.cursor != 0 {
		t.Errorf("f a collapse: expected cursor 0 on parent, got %d", m.cursor)
	}

	// Second f a: restore the subtree.
	m.focusBoardToggleFold()
	if len(m.filtered) != 3 {
		t.Fatalf("f a restore: expected 3 rows, got %d", len(m.filtered))
	}
	if m.filtered[m.cursor].Task.ID != "id-parent" {
		t.Errorf("f a restore: cursor should stay on parent, got %q", m.filtered[m.cursor].Task.ID)
	}
}

// TestFocusBoardToggleFoldBeatsDepth verifies f a re-expands a node hidden by a
// prior depth command (per-node override beats the depth limit).
func TestFocusBoardToggleFoldBeatsDepth(t *testing.T) {
	m := setupFoldBoard(t)

	// f 1 (limit 0): collapse everything to roots only.
	m.focusBoardCollapseToDepthAction(0)()
	if len(m.filtered) != 1 {
		t.Fatalf("f 1: expected 1 row, got %d", len(m.filtered))
	}

	// f a on the parent re-expands it despite the depth limit.
	m.cursor = 0
	m.focusBoardToggleFold()
	if len(m.filtered) != 2 {
		t.Fatalf("f a beats depth: expected 2 rows (parent + one child level), got %d", len(m.filtered))
	}
	if m.foldOverrides["id-parent"] {
		t.Errorf("f a beats depth: parent override should force-expand (false), got true")
	}
}

// TestFocusBoardDepthClearsFoldOverride verifies a depth command clears prior f a
// overrides for a uniform reset.
func TestFocusBoardDepthClearsFoldOverride(t *testing.T) {
	m := setupFoldBoard(t)

	// Collapse the parent via f a, establishing an override.
	m.cursor = 0
	m.focusBoardToggleFold()
	if len(m.foldOverrides) != 1 {
		t.Fatalf("setup: expected 1 override after f a, got %d", len(m.foldOverrides))
	}

	// A depth command (f 3 → limit 2) clears overrides and shows the full tree.
	m.focusBoardCollapseToDepthAction(2)()
	if len(m.foldOverrides) != 0 {
		t.Errorf("depth reset: expected foldOverrides cleared, got %d entries", len(m.foldOverrides))
	}
	if len(m.filtered) != 3 {
		t.Fatalf("depth reset: expected 3 rows after clearing override, got %d", len(m.filtered))
	}
}
