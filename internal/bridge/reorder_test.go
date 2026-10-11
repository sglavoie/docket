package bridge

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
)

func TestPlanningDragPreservesUnnamedSlots(t *testing.T) {
	members := []config.ProjectTaskLocator{
		{ProjectID: "p", TaskID: "a"},
		{ProjectID: "parked", TaskID: "hidden"},
		{ProjectID: "p", TaskID: "b"},
		{ProjectID: "p", TaskID: "new"},
	}
	got, err := mergePlanningOrder(members, []app.TaskLocator{
		{ProjectID: "p", TaskID: "b"}, {ProjectID: "p", TaskID: "a"},
	})
	want := []app.TaskLocator{
		{ProjectID: "p", TaskID: "b"}, {ProjectID: "parked", TaskID: "hidden"},
		{ProjectID: "p", TaskID: "a"}, {ProjectID: "p", TaskID: "new"},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("merged order = %v, %v; want %v", got, err, want)
	}
}

func TestBridgeReorderPlanningPersistsAcrossPriorities(t *testing.T) {
	for _, kind := range []string{"today", "next"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t, representativeFixture)
			h.open(false)
			ship, tiles := task(atlasProject, shipBetaTask), task(homeProject, tilesTask)
			for _, target := range []locator{ship, tiles} {
				h.ok(h.call("command.plan", map[string]any{"task": target, "action": "add_" + kind}), nil)
			}
			var result tasksResultView
			h.ok(h.call("command.reorder", reorderPayload{Task: tiles, Kind: kind, Order: []locator{tiles, ship}}), &result)
			if len(result.Committed.Domains) == 0 || result.Committed.TransactionID == "" {
				t.Fatal("reorder reported no planning transaction")
			}
			h.ok(h.call("workspace.close", nil), nil)
			h.open(false)
			got := h.titles(map[string]any{"mode": kind, "sort": map[string]any{"field": "manual", "ascending": true}})
			if slices.Index(got, "Order bathroom tiles") >= slices.Index(got, "Ship beta build") {
				t.Fatalf("manual order after reopen: %v", got)
			}
			if kind == "today" {
				var widget widgetSnapshotResult
				h.ok(h.call("query.widget_snapshot", nil), &widget)
				if len(widget.Today.Tasks) < 2 || widget.Today.Tasks[0].TaskID != tilesTask {
					t.Fatalf("widget ignored the saved order: %+v", widget.Today.Tasks)
				}
			}
		})
	}
}

func TestBridgeReorderSiblingsAndRefusals(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	ship, waiting := task(atlasProject, shipBetaTask), task(atlasProject, signingTask)
	h.ok(h.call("command.reorder", reorderPayload{Task: waiting, Kind: "siblings", Order: []locator{waiting, ship}}), nil)
	got := h.titles(map[string]any{"mode": "all", "scope": atlasProject, "sort": map[string]any{"field": "manual"}})
	if len(got) == 0 || got[0] != "Await signing certificate" {
		t.Fatalf("manual sibling order: %v", got)
	}
	if detail := h.detail(atlasProject, releaseNotesTask); detail.Task.ParentID != shipBetaTask {
		t.Fatal("reorder changed the subtask's parent")
	}
	for _, payload := range []reorderPayload{
		{Task: ship, Kind: "siblings", Order: []locator{ship, ship}},
		{Task: ship, Kind: "siblings", Order: []locator{ship, task(homeProject, tilesTask)}},
		{Task: ship, Kind: "siblings", Order: []locator{ship, task(atlasProject, releaseNotesTask)}},
		{Task: waiting, Kind: "today", Order: []locator{waiting, ship}},
	} {
		h.unchanged("invalid reorder", func() { h.rejects(h.call("command.reorder", payload), CodeValidation) })
	}
}
