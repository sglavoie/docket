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
	planning.SetNext(input)
	planning.SetToday(input)

	want := []ProjectTaskLocator{
		{TaskID: "same", ProjectID: "p"},
		{TaskID: "same", ProjectID: "other"},
	}
	if got := planning.GetNext(); !planningRefsEqual(got, want) {
		t.Fatalf("NextOrder = %+v, want %+v", got, want)
	}
	if got := planning.GetToday(); !planningRefsEqual(got, want) {
		t.Fatalf("Today = %+v, want %+v", got, want)
	}
}

func TestPlanningMutationsRequireExactProjectQualifiedIdentity(t *testing.T) {
	planning := NewPlanningState()
	if !planning.AddToNext("shared", "alpha") ||
		!planning.AddToNext("shared", "beta") ||
		planning.AddToNext("shared", "alpha") {
		t.Fatalf("Next-order additions were not project-qualified: %+v", planning.Next)
	}
	if !planning.AddToToday("shared", "alpha") ||
		!planning.AddToToday("shared", "beta") ||
		planning.AddToToday("shared", "alpha") {
		t.Fatalf("Today additions were not project-qualified: %+v", planning.Today)
	}

	if !planning.RemoveFromNextInProject("shared", "beta") ||
		!planning.RemoveFromTodayInProject("shared", "beta") {
		t.Fatal("qualified removals reported no change")
	}
	if got := planning.Next; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("NextOrder = %+v, want alpha entry preserved", got)
	}
	if got := planning.Today; len(got) != 1 || got[0].ProjectID != "alpha" {
		t.Fatalf("Today = %+v, want alpha entry preserved", got)
	}
}

func TestPlanningQualifiedCleanupAndMovePreserveOtherProject(t *testing.T) {
	planning := NewPlanningState()
	planning.SetNext([]ProjectTaskLocator{
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
	planning.AddToNext("shared", "beta")
	planning.AddToToday("shared", "beta")
	if updated := planning.ReassignDurablePlanningProjectFrom(
		map[string]bool{"shared": true}, "beta", "gamma",
	); updated != 2 {
		t.Fatalf("updated = %d, want two planning entries", updated)
	}
	if planning.Next[0].ProjectID != "alpha" || planning.Next[1].ProjectID != "gamma" {
		t.Fatalf("NextOrder = %+v", planning.Next)
	}
	if planning.Today[0].ProjectID != "alpha" || planning.Today[1].ProjectID != "gamma" {
		t.Fatalf("Today = %+v", planning.Today)
	}
}

func TestCanonicalV6PlanningJSONContainsOnlyExplicitMemberships(t *testing.T) {
	planning := PlanningState{
		Today: []ProjectTaskLocator{{TaskID: "today", ProjectID: "p"}},
		Next:  []ProjectTaskLocator{{TaskID: "next", ProjectID: "p"}},
	}
	output, err := json.Marshal(planning)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if got, want := string(output), `{"today":[{"task_id":"today","project_id":"p"}],"next":[{"task_id":"next","project_id":"p"}]}`; got != want {
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
