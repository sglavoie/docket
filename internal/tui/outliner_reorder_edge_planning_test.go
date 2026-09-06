package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestPlanningReorderEdge(t *testing.T) {
	t.Run("Ctrl+J sends the member to the end of the focus queue", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "pl-down", "pl-down",
			"Alpha", "Bravo", "Charlie", "Delta")
		m, cross, dataDir := newPlanningOrderApp(t, project)
		seedNextOrderOrder(t, m,
			locatorOf(project, tasks[0]), locatorOf(project, tasks[1]),
			locatorOf(project, tasks[2]), locatorOf(project, tasks[3]),
		)
		enterCrossMode(cross, chrome.ModeNext)
		if !cross.MoveCursorToProjectTask(tasks[0].ID, project.ID) {
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
		enterCrossMode(cross, chrome.ModeToday)
		if !cross.MoveCursorToProjectTask(tasks[2].ID, project.ID) {
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
		cross.Filter.Tags = map[string]bool{"keep": true}
		adoptFilterCarriers(cross)
		enterCrossMode(cross, chrome.ModeNext)
		if got := visibleTaskIDs(cross); len(got) != 3 {
			t.Fatalf("filtered Next rows = %v, want 3", got)
		}
		if !cross.MoveCursorToProjectTask(tasks[0].ID, project.ID) {
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
		cross.Filter.Tags = map[string]bool{"keep": true}
		adoptFilterCarriers(cross)
		enterCrossMode(cross, chrome.ModeNext)
		if !cross.MoveCursorToProjectTask(tasks[0].ID, project.ID) {
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
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks[1].ID, project.ID) {
			t.Fatal("cursor did not land on Today Two")
		}
		if !cross.Rows[cross.Cursor].IsTodaySection {
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
		enterCrossMode(cross, chrome.ModeToday)

		if !cross.MoveCursorToProjectTask(bravoTasks[0].ID, bravo.ID) {
			t.Fatal("cursor did not land on B1")
		}
		if pressReorderCtrlKey(t, m, 'k') {
			t.Fatal("Ctrl+K lifted the second project's group above the first")
		}
		assertOrder(t, "daily plan", diskTodayIDs(t, dataDir), seeded)

		// Inside the group the edge move still works.
		if !cross.MoveCursorToProjectTask(bravoTasks[1].ID, bravo.ID) {
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
		enterCrossMode(cross, chrome.ModeNext)
		if !cross.MoveCursorToProjectTask(tasks[0].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}
		cross.ToggleMultiSelectMode()

		if pressReorderCtrlKey(t, m, 'j') {
			t.Fatal("Ctrl+J reordered a planning list under a visual selection")
		}
		assertOrder(t, "focus queue", diskNextOrderIDs(t, dataDir),
			[]string{tasks[0].ID, tasks[1].ID, tasks[2].ID})
	})
}
