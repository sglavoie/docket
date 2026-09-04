package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The Ctrl+J/Ctrl+K edge moves are the single-press equivalent of repeating J/K
// until the row can move no further, so every test below states the order a run
// of J presses would have produced and proves one press reaches it — over one
// transaction, and without disturbing a peer the projection hides.

func TestHierarchicalReorderEdge(t *testing.T) {
	t.Run("Ctrl+J from the middle lands the task last among its siblings", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-down", "h-down",
			"Alpha", "Bravo", "Charlie", "Delta")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		if !cross.MoveCursorToProjectTask(ids[1], project.ID) {
			t.Fatal("cursor did not land on Bravo")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on a hierarchical row produced no reorder command")
		}
		if m.feedback.err != nil {
			t.Fatalf("hierarchical edge reorder failed: %v", m.feedback.err)
		}
		assertTreeUnchanged(t, project, []string{ids[0], ids[2], ids[3], ids[1]})
	})

	t.Run("Ctrl+K from the middle lands the task first among its siblings", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-up", "h-up",
			"Alpha", "Bravo", "Charlie", "Delta")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		if !cross.MoveCursorToProjectTask(ids[2], project.ID) {
			t.Fatal("cursor did not land on Charlie")
		}

		if !pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K on a hierarchical row produced no reorder command")
		}
		assertTreeUnchanged(t, project, []string{ids[2], ids[0], ids[1], ids[3]})
	})

	t.Run("Ctrl+J is a no-op on the last sibling", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-edge", "h-edge",
			"Alpha", "Bravo")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		if !cross.MoveCursorToProjectTask(ids[1], project.ID) {
			t.Fatal("cursor did not land on Bravo")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on the last sibling emitted a reorder command")
		}
		assertTreeUnchanged(t, project, []string{ids[0], ids[1]})
	})

	t.Run("a filtered sibling keeps its own slot", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-filter", "h-filter",
			"keep-one", "hide-me", "keep-two", "keep-three")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		for _, index := range []int{0, 2, 3} {
			tasks[index].Tags = []string{"keep"}
		}
		cross.Filter.Tags = map[string]bool{"keep": true}
		cross.AdoptFilterCarriers()
		cross.Population.Invalidate()
		cross.CrossProjectRebuildRows()
		if got := visibleTaskIDs(cross); len(got) != 3 {
			t.Fatalf("filtered rows = %v, want 3 (hidden sibling pruned)", got)
		}
		if !cross.MoveCursorToProjectTask(ids[0], project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J in a filtered projection produced no reorder command")
		}
		// Only the three visible siblings' slots (0, 2, 3) are rewritten, so
		// hide-me is still the second task on disk.
		assertTreeUnchanged(t, project, []string{ids[2], ids[1], ids[3], ids[0]})
	})

	t.Run("Ctrl+J stops at the project group boundary", func(t *testing.T) {
		dir := t.TempDir()
		alpha, alphaTasks := planningOrderProject(t, dir, "h-alpha", "h-alpha", "Alpha One", "Alpha Two")
		beta, betaTasks := planningOrderProject(t, dir, "h-beta", "h-beta", "Beta One", "Beta Two")
		alphaIDs, betaIDs := seededTaskIDs(alphaTasks), seededTaskIDs(betaTasks)
		m, cross := newHierarchyEdgeApp(t, alpha, beta)
		if !cross.MoveCursorToProjectTask(alphaIDs[1], alpha.ID) {
			t.Fatal("cursor did not land on Alpha Two")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J crossed the project group boundary")
		}
		assertTreeUnchanged(t, alpha, alphaIDs)
		assertTreeUnchanged(t, beta, betaIDs)
	})

	t.Run("a visual selection refuses the edge move", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-visual", "h-visual",
			"Alpha", "Bravo", "Charlie")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		if !cross.MoveCursorToProjectTask(ids[0], project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.ToggleMultiSelectMode()
		if !cross.MultiSelectMode {
			t.Fatal("visual selection did not open")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J reordered under a visual selection")
		}
		assertTreeUnchanged(t, project, ids)
	})

	t.Run("the whole edge move rides one undo entry", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-undo", "h-undo",
			"Alpha", "Bravo", "Charlie", "Delta")
		ids := seededTaskIDs(tasks)
		m, cross, dataDir := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeBacklog)
		if !cross.MoveCursorToProjectTask(ids[0], project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J produced no reorder command")
		}
		assertTreeUnchanged(t, project, []string{ids[1], ids[2], ids[3], ids[0]})

		ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
		if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
			t.Fatalf("ledger undo: %v", err)
		}
		assertTreeUnchanged(t, project, ids)
	})
}

func TestPinnedReorderEdge(t *testing.T) {
	base := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)

	t.Run("Ctrl+J sends the pin past every visible pin below it", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-edge-down", "edge-down",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
			pinSpec{title: "Charlie", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on a pinned row produced no reorder command")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross), []string{"Bravo", "Charlie", "Alpha"})
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Bravo", "Charlie", "Alpha"})
	})

	t.Run("Ctrl+K sends the pin above every visible pin", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-edge-up", "edge-up",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
			pinSpec{title: "Charlie", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Charlie"].ID, project.ID) {
			t.Fatal("cursor did not land on Charlie")
		}

		if !pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K on a pinned row produced no reorder command")
		}
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Charlie", "Alpha", "Bravo"})
	})

	t.Run("Ctrl+J is a no-op on the last pin of the group", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-edge-last", "edge-last",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Bravo"].ID, project.ID) {
			t.Fatal("cursor did not land on Bravo")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on the last pin emitted a reorder command")
		}
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Alpha", "Bravo"})
	})

	t.Run("Ctrl+J stops at the pinned project group boundary", func(t *testing.T) {
		alpha, alphaTasks := pinnedOrderProject(t, "p-edge-alpha", "edge-alpha",
			pinSpec{title: "Alpha One", pinnedAt: base},
			pinSpec{title: "Alpha Two", pinnedAt: base.Add(-time.Hour)},
		)
		beta, _ := pinnedOrderProject(t, "p-edge-beta", "edge-beta",
			pinSpec{title: "Beta One", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, alpha, beta)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(alphaTasks["Alpha Two"].ID, alpha.ID) {
			t.Fatal("cursor did not land on Alpha Two")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J crossed the Pinned project group boundary")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross),
			[]string{"Alpha One", "Alpha Two", "Beta One"})
	})

	t.Run("a nested pin reaches only its own siblings", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-edge-nest", "edge-nest",
			pinSpec{title: "Parent", pinnedAt: base},
			pinSpec{title: "First Child", pinnedAt: base.Add(-time.Hour), parent: "Parent"},
			pinSpec{title: "Second Child", pinnedAt: base.Add(-2 * time.Hour), parent: "Parent"},
			pinSpec{title: "Third Child", pinnedAt: base.Add(-3 * time.Hour), parent: "Parent"},
			pinSpec{title: "Other Root", pinnedAt: base.Add(-4 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["First Child"].ID, project.ID) {
			t.Fatal("cursor did not land on First Child")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on a nested pin produced no reorder command")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross),
			[]string{"Parent", "Second Child", "Third Child", "First Child", "Other Root"})
	})

	t.Run("a visual selection refuses the edge move", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-edge-visual", "edge-visual",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.ToggleMultiSelectMode()

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J reordered a pin under a visual selection")
		}
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Alpha", "Bravo"})
	})
}

// seededTaskIDs freezes the seeded task IDs in their pre-reorder order, because
// planningOrderProject hands back the very slice the project reorders in place.
func seededTaskIDs(tasks []*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

// newHierarchyEdgeApp builds the planning-order app wired for hierarchical
// reorders and parks it in Backlog, the mode whose rows carry manual order.
func newHierarchyEdgeApp(t *testing.T, projects ...*models.Project) (*AppModel, *outliner.OutlinerModel) {
	t.Helper()
	m, cross, _ := newPlanningOrderApp(t, projects...)
	cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	enterCrossMode(cross, chrome.ModeBacklog)
	return m, cross
}
