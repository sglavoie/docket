package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func plannedRuntime(t *testing.T, parkedTags ...string) *RuntimeState {
	t.Helper()
	catalog := NewWorkspaceCatalog()
	catalog.Projects = []CatalogProject{
		{ID: "plain", Slug: "plain", Name: "Plain", ColorSlot: 1},
		{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 2, Tags: parkedTags},
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("planning fixture must be a valid catalog: %v", err)
	}
	planning := NewPlanningState()
	planning.Next = []ProjectTaskLocator{
		{ProjectID: "plain", TaskID: "t1"},
		{ProjectID: "parked", TaskID: "t2"},
		{ProjectID: "plain", TaskID: "t3"},
	}
	return NewRuntimeState(t.TempDir(), &Domains{
		Catalog:  catalog,
		Planning: planning,
		Machine:  NewMachineState(),
	})
}

func locatorTaskIDs(items []ProjectTaskLocator) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func assertTaskIDs(t *testing.T, label string, got []ProjectTaskLocator, want []string) {
	t.Helper()
	ids := locatorTaskIDs(got)
	if len(ids) != len(want) {
		t.Fatalf("%s = %v, want %v", label, ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, ids, want)
		}
	}
}

// Parking is a read-time predicate, not a destructive edit: the parked owner's
// Next locator stays on disk and only disappears from the filtered accessor.
func TestNextOrderHidesParkedOwnersWithoutEditingPlanning(t *testing.T) {
	rs := plannedRuntime(t, models.ParkedProjectTag)

	assertTaskIDs(t, "NextOrder", rs.NextOrder(), []string{"t1", "t3"})
	assertTaskIDs(t, "GetNext", rs.GetNext(), []string{"t1", "t3"})
	assertTaskIDs(t, "Planning().GetNext", rs.Planning().GetNext(), []string{"t1", "t2", "t3"})
	if err := rs.Planning().Validate(); err != nil {
		t.Fatalf("planning holding a parked owner's entry must stay valid: %v", err)
	}
}

// Unparking restores the plan untouched because nothing was ever removed.
func TestNextOrderRestoresPlanWhenOwnerIsUnparked(t *testing.T) {
	rs := plannedRuntime(t)
	assertTaskIDs(t, "NextOrder", rs.NextOrder(), []string{"t1", "t2", "t3"})
}
