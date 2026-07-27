package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplySyncedProjectReferencesKeepsDuplicateTaskIDsAcrossProjects(t *testing.T) {
	domains := NewDomains()
	domains.Planning.FocusQueue = []FocusQueueItem{{ProjectID: "beta", TaskID: "shared"}}
	domains.Planning.DailyPlan = []DailyPlanItem{{ProjectID: "beta", TaskID: "shared", PlannedDate: "2026-07-22"}}
	refs := ProjectReferences{
		FocusQueue: []FocusQueueItem{{ProjectID: "archived-alpha", TaskID: "shared"}},
		DailyPlan:  []DailyPlanItem{{ProjectID: "archived-alpha", TaskID: "shared", PlannedDate: "2026-07-22"}},
	}

	domains.ApplySyncedProjectReferences("alpha", refs)
	domains.ApplySyncedProjectReferences("alpha", refs)

	if len(domains.Planning.FocusQueue) != 2 {
		t.Fatalf("Focus Queue = %+v, want one alpha and one beta entry", domains.Planning.FocusQueue)
	}
	if len(domains.Planning.DailyPlan) != 2 {
		t.Fatalf("Daily Plan = %+v, want one alpha and one beta entry", domains.Planning.DailyPlan)
	}
	if domains.Planning.FocusQueue[1].ProjectID != "alpha" || domains.Planning.DailyPlan[1].ProjectID != "alpha" {
		t.Fatalf("restored ownership was not reassigned to alpha: queue=%+v plan=%+v", domains.Planning.FocusQueue, domains.Planning.DailyPlan)
	}
}

func TestApplyProjectReferencesKeepsDuplicateRecentVisitIDsAcrossProjects(t *testing.T) {
	machine := &MachineState{
		RecentTaskVisits: []RecentTaskVisit{{ProjectID: "beta", TaskID: "shared"}},
	}
	refs := ProjectReferences{
		RecentTaskVisits: []RecentTaskVisit{{ProjectID: "archived-alpha", TaskID: "shared"}},
	}

	machine.ApplyProjectReferences("alpha", refs)
	machine.ApplyProjectReferences("alpha", refs)

	if len(machine.RecentTaskVisits) != 2 {
		t.Fatalf("RecentTaskVisits = %+v, want one alpha and one beta visit", machine.RecentTaskVisits)
	}
	visits := make(map[ProjectTaskLocator]bool, len(machine.RecentTaskVisits))
	for _, visit := range machine.RecentTaskVisits {
		visits[ProjectTaskLocator{ProjectID: visit.ProjectID, TaskID: visit.TaskID}] = true
	}
	for _, locator := range []ProjectTaskLocator{
		{ProjectID: "alpha", TaskID: "shared"},
		{ProjectID: "beta", TaskID: "shared"},
	} {
		if !visits[locator] {
			t.Errorf("RecentTaskVisits = %+v, missing %+v", machine.RecentTaskVisits, locator)
		}
	}
}

func TestProjectReferencesRoundTripOutlinerRouteUnderCanonicalProjectID(t *testing.T) {
	runtime := NewRuntimeState("", NewDomains())
	machine := runtime.Machine()
	machine.ProjectRoutes = map[string]OutlinerRouteState{
		"archived-alpha": {Mode: "waiting"},
	}
	runtime.SetMachineInMemory(machine)

	refs := runtime.ExtractProjectReferences("archived-alpha", "")
	if refs.OutlinerRoute == nil ||
		refs.OutlinerRoute.Mode != OutlinerModeAll ||
		!refs.OutlinerRoute.FilterStates.Contains(models.StateWaiting) {
		t.Fatalf("extracted route = %+v", refs.OutlinerRoute)
	}

	restored := NewMachineState()
	if !restored.ApplyProjectReferences("alpha", refs) {
		t.Fatal("restoring route reported no change")
	}
	if _, stale := restored.ProjectRoutes["archived-alpha"]; stale {
		t.Fatalf("restored stale route key: %+v", restored.ProjectRoutes)
	}
	route, ok := restored.ProjectRoutes["alpha"]
	if !ok || route.Mode != OutlinerModeAll || !route.FilterStates.Contains(models.StateWaiting) {
		t.Fatalf("restored route = %+v", restored.ProjectRoutes)
	}
}
