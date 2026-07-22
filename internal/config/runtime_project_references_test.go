package config

import "testing"

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
