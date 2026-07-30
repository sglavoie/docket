package config

import (
	"encoding/json"
	"testing"
)

func TestPlanningSettersCanonicalizeProjectQualifiedIdentities(t *testing.T) {
	planning := NewPlanningState()
	input := []ProjectTaskLocator{
		{TaskID: "", ProjectID: "p"},
		{TaskID: "missing-project"},
		{TaskID: "same", ProjectID: "p"},
		{TaskID: "same", ProjectID: "p"},
		{TaskID: "same", ProjectID: "other"},
	}
	planning.SetNextOrder(input)
	planning.SetToday(input)

	want := []ProjectTaskLocator{
		{TaskID: "same", ProjectID: "p"},
		{TaskID: "same", ProjectID: "other"},
	}
	if got := planning.GetNextOrder(); !planningRefsEqual(got, want) {
		t.Fatalf("NextOrder = %+v, want %+v", got, want)
	}
	if got := planning.GetToday(); !planningRefsEqual(got, want) {
		t.Fatalf("Today = %+v, want %+v", got, want)
	}
}

func TestPlanningMutationsRequireExactProjectQualifiedIdentity(t *testing.T) {
	planning := NewPlanningState()
	if !planning.AddToNextOrder("shared", "alpha") ||
		!planning.AddToNextOrder("shared", "beta") ||
		planning.AddToNextOrder("shared", "alpha") {
		t.Fatalf("Next-order additions were not project-qualified: %+v", planning.NextOrder)
	}
	if !planning.AddToToday("shared", "alpha") ||
		!planning.AddToToday("shared", "beta") ||
		planning.AddToToday("shared", "alpha") {
		t.Fatalf("Today additions were not project-qualified: %+v", planning.Today)
	}

	if !planning.RemoveFromNextOrderInProject("shared", "beta") ||
		!planning.RemoveFromTodayInProject("shared", "beta") {
		t.Fatal("qualified removals reported no change")
	}
	if got := planning.NextOrder; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("NextOrder = %+v, want alpha entry preserved", got)
	}
	if got := planning.Today; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("Today = %+v, want alpha entry preserved", got)
	}
}

func TestPlanningQualifiedCleanupAndMovePreserveOtherProject(t *testing.T) {
	planning := NewPlanningState()
	planning.SetNextOrder([]ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared"},
		{ProjectID: "beta", TaskID: "shared"},
	})
	planning.SetToday([]ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared"},
		{ProjectID: "beta", TaskID: "shared"},
	})

	removed := planning.RemoveDurableTaskLocators([]ProjectTaskLocator{{
		ProjectID: "beta", TaskID: "shared",
	}})
	if removed != 2 {
		t.Fatalf("removed = %d, want beta Today + Next-order entries", removed)
	}
	planning.AddToNextOrder("shared", "beta")
	planning.AddToToday("shared", "beta")
	if updated := planning.ReassignDurablePlanningProjectFrom(
		map[string]bool{"shared": true}, "beta", "gamma",
	); updated != 2 {
		t.Fatalf("updated = %d, want two planning entries", updated)
	}
	if planning.NextOrder[0].ProjectID != "alpha" || planning.NextOrder[1].ProjectID != "gamma" {
		t.Fatalf("NextOrder = %+v", planning.NextOrder)
	}
	if planning.Today[0].ProjectID != "alpha" || planning.Today[1].ProjectID != "gamma" {
		t.Fatalf("Today = %+v", planning.Today)
	}
}

func TestCanonicalV5PlanningJSONContainsOnlyLeanFields(t *testing.T) {
	planning := PlanningState{
		Today:     []ProjectTaskLocator{{TaskID: "today", ProjectID: "p"}},
		NextOrder: []ProjectTaskLocator{{TaskID: "next", ProjectID: "p"}},
	}
	output, err := json.Marshal(planning)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if got, want := string(output), `{"today":[{"task_id":"today","project_id":"p"}],"next_order":[{"task_id":"next","project_id":"p"}]}`; got != want {
		t.Fatalf("canonical planning JSON = %s, want %s", got, want)
	}
}

func planningRefsEqual(left, right []ProjectTaskLocator) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
