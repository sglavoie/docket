package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
		if !cross.moveCursorToProjectTask(ids[1], project.ID) {
			t.Fatal("cursor did not land on Bravo")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J on a hierarchical row produced no reorder command")
		}
		if m.err != nil {
			t.Fatalf("hierarchical edge reorder failed: %v", m.err)
		}
		assertTreeUnchanged(t, project, []string{ids[0], ids[2], ids[3], ids[1]})
	})

	t.Run("Ctrl+K from the middle lands the task first among its siblings", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "h-up", "h-up",
			"Alpha", "Bravo", "Charlie", "Delta")
		ids := seededTaskIDs(tasks)
		m, cross := newHierarchyEdgeApp(t, project)
		if !cross.moveCursorToProjectTask(ids[2], project.ID) {
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
		if !cross.moveCursorToProjectTask(ids[1], project.ID) {
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
		cross.filter.Tags = map[string]bool{"keep": true}
		cross.crossForceRebuild = true
		cross.crossProjectRebuildRows()
		if got := visibleTaskIDs(cross); len(got) != 3 {
			t.Fatalf("filtered rows = %v, want 3 (hidden sibling pruned)", got)
		}
		if !cross.moveCursorToProjectTask(ids[0], project.ID) {
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
		if !cross.moveCursorToProjectTask(alphaIDs[1], alpha.ID) {
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
		if !cross.moveCursorToProjectTask(ids[0], project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.toggleMultiSelectMode()
		if !cross.multiSelectMode {
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
		enterCrossMode(cross, ModeBacklog)
		if !cross.moveCursorToProjectTask(ids[0], project.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks["Charlie"].ID, project.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks["Bravo"].ID, project.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(alphaTasks["Alpha Two"].ID, alpha.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks["First Child"].ID, project.ID) {
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
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.toggleMultiSelectMode()

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J reordered a pin under a visual selection")
		}
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Alpha", "Bravo"})
	})
}

func TestPlanningReorderEdge(t *testing.T) {
	t.Run("Ctrl+J sends the member to the end of the focus queue", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-down", "pl-down",
			"Alpha", "Bravo", "Charlie", "Delta")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]),
			locatorOf(project, tasks[2]), locatorOf(project, tasks[3]),
		)
		enterCrossMode(cross, ModeNext)
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J in Next produced no reorder command")
		}
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir),
			[]string{tasks[1].ID, tasks[2].ID, tasks[3].ID, tasks[0].ID})
		assertTreeUnchanged(t, project,
			[]string{tasks[0].ID, tasks[1].ID, tasks[2].ID, tasks[3].ID})
	})

	t.Run("Ctrl+K sends the member to the head of the daily plan", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-up", "pl-up",
			"Alpha", "Bravo", "Charlie")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedTodayOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		enterCrossMode(cross, ModeToday)
		if !cross.moveCursorToProjectTask(tasks[2].ID, project.ID) {
			t.Fatal("cursor did not land on Charlie")
		}

		if !pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K in Today produced no reorder command")
		}
		assertOrder(t, "daily plan", diskTodayIDs(t, dataDir),
			[]string{tasks[2].ID, tasks[0].ID, tasks[1].ID})
	})

	t.Run("a filtered planning member keeps its own position", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-filter", "pl-filter",
			"keep-one", "hide-me", "keep-two", "keep-three")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]),
			locatorOf(project, tasks[2]), locatorOf(project, tasks[3]),
		)
		for _, index := range []int{0, 2, 3} {
			tasks[index].Tags = []string{"keep"}
		}
		cross.filter.Tags = map[string]bool{"keep": true}
		enterCrossMode(cross, ModeNext)
		if got := visibleTaskIDs(cross); len(got) != 3 {
			t.Fatalf("filtered Next rows = %v, want 3", got)
		}
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if !pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J in filtered Next produced no reorder command")
		}
		// Positions 0, 2 and 3 are rotated; the hidden member keeps position 1.
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir),
			[]string{tasks[2].ID, tasks[1].ID, tasks[3].ID, tasks[0].ID})
	})

	t.Run("Ctrl+J is a no-op at the visible boundary", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-bound", "pl-bound",
			"keep-one", "hide-me", "hide-two")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		tasks[0].Tags = []string{"keep"}
		cross.filter.Tags = map[string]bool{"keep": true}
		enterCrossMode(cross, ModeNext)
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on keep-one")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J past the last visible member emitted a reorder command")
		}
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir),
			[]string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})

	t.Run("Ctrl+J stops at the All-mode planning section boundary", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-section", "pl-section",
			"Today One", "Today Two", "Next One", "Next Two")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedTodayOrder(t, m, locatorOf(project, tasks[0]), locatorOf(project, tasks[1]))
		seedNextOrderOrder(t, m, locatorOf(project, tasks[2]), locatorOf(project, tasks[3]))
		enterCrossMode(cross, ModeAll)
		if !cross.moveCursorToProjectTask(tasks[1].ID, project.ID) {
			t.Fatal("cursor did not land on Today Two")
		}
		if !cross.rows[cross.cursor].IsTodaySection {
			t.Fatal("the cursor row lost its Today section identity")
		}

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J crossed from the Today section into Next")
		}
		assertOrder(t, "daily plan", diskTodayIDs(t, dataDir), []string{tasks[0].ID, tasks[1].ID})
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir), []string{tasks[2].ID, tasks[3].ID})
	})

	// Today and Next are project-qualified, and their group order follows the
	// durable order itself, so a run that reached into the neighboring group
	// would hoist the cursor's whole project above it. The edge move stops at
	// its group even though a single J/K step may still cross one.
	t.Run("Ctrl+K stops at the planning project group", func(t *testing.T) {
		dir := t.TempDir()
		alpha, alphaTasks := planningOrderProject(t, dir, "pl-group-a", "pl-group-a", "A1", "A2")
		bravo, bravoTasks := planningOrderProject(t, dir, "pl-group-b", "pl-group-b", "B1", "B2")
		m, cross, dataDir := newPlanningOrderApp(t, alpha, bravo)
		seedTodayOrder(t, m,
			locatorOf(alpha, alphaTasks[0]), locatorOf(alpha, alphaTasks[1]),
			locatorOf(bravo, bravoTasks[0]), locatorOf(bravo, bravoTasks[1]),
		)
		seeded := []string{alphaTasks[0].ID, alphaTasks[1].ID, bravoTasks[0].ID, bravoTasks[1].ID}
		enterCrossMode(cross, ModeToday)

		if !cross.moveCursorToProjectTask(bravoTasks[0].ID, bravo.ID) {
			t.Fatal("cursor did not land on B1")
		}
		if pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K lifted the second project's group above the first")
		}
		assertOrder(t, "daily plan", diskTodayIDs(t, dataDir), seeded)

		// Inside the group the edge move still works.
		if !cross.moveCursorToProjectTask(bravoTasks[1].ID, bravo.ID) {
			t.Fatal("cursor did not land on B2")
		}
		if !pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K inside the group produced no reorder command")
		}
		assertOrder(t, "daily plan", diskTodayIDs(t, dataDir),
			[]string{alphaTasks[0].ID, alphaTasks[1].ID, bravoTasks[1].ID, bravoTasks[0].ID})
	})

	t.Run("a visual selection refuses the edge move", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-visual", "pl-visual",
			"Alpha", "Bravo", "Charlie")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]), locatorOf(project, tasks[2]),
		)
		enterCrossMode(cross, ModeNext)
		if !cross.moveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.toggleMultiSelectMode()

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J reordered a planning list under a visual selection")
		}
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir),
			[]string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})
}

func TestChecklistReorderEdge(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	outliner := driver.App().crossOutliner
	focusChecklistOwner(t, driver, checklistFixtureFoldTitle)
	if !outliner.expandAtCursor() {
		t.Fatal("expandAtCursor did not expand the checklist")
	}
	outliner.cursor++
	if !outliner.rows[outliner.cursor].IsChecklistItem {
		t.Fatal("the cursor is not on the first checklist row")
	}

	pressChecklistEdgeKey(t, driver, "ctrl+j")
	assertOrder(t, "checklist after Ctrl+J", checklistRowTexts(outliner), []string{
		checklistFixtureSecondItem, checklistFixtureThirdItem, checklistFixtureFirstItem,
	})
	if !outliner.rows[outliner.cursor].IsChecklistItem ||
		outliner.rows[outliner.cursor].ChecklistIndex != 2 {
		t.Fatalf("cursor did not follow the moved item: %+v", outliner.rows[outliner.cursor])
	}

	// The item is already last, so a second Ctrl+J changes nothing.
	pressChecklistEdgeKey(t, driver, "ctrl+j")
	assertOrder(t, "checklist after the no-op Ctrl+J", checklistRowTexts(outliner), []string{
		checklistFixtureSecondItem, checklistFixtureThirdItem, checklistFixtureFirstItem,
	})

	pressChecklistEdgeKey(t, driver, "ctrl+k")
	assertOrder(t, "checklist after Ctrl+K", checklistRowTexts(outliner), []string{
		checklistFixtureFirstItem, checklistFixtureSecondItem, checklistFixtureThirdItem,
	})
	if outliner.rows[outliner.cursor].ChecklistIndex != 0 {
		t.Fatalf("cursor did not follow the item back to the top: %+v", outliner.rows[outliner.cursor])
	}
}

// pressChecklistEdgeKey presses one key and settles the transaction it starts.
// A checklist move writes the project document, which routinely outlasts the
// driver's per-command drain budget and is abandoned to the pending set.
func pressChecklistEdgeKey(t *testing.T, driver *UITestDriver, key string) {
	t.Helper()
	sortPressKeys(t, driver, key)
	driver.Wait(defaultWaitTimeout)
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
func newHierarchyEdgeApp(t *testing.T, projects ...*models.Project) (*AppModel, *OutlinerModel) {
	t.Helper()
	m, cross, _ := newPlanningOrderApp(t, projects...)
	cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	enterCrossMode(cross, ModeBacklog)
	return m, cross
}
