package query

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestSearchExactDaysMatchPersistedQueriesAcrossZonesAndDST(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	for _, loc := range []*time.Location{time.FixedZone("west", -10*3600), time.FixedZone("east", 14*3600), havana} {
		now := time.Date(2026, time.March, 9, 12, 0, 0, 0, loc)
		for _, input := range []string{"@2026-03-08", "@@2026-03-08"} {
			persisted, remainder := models.PartitionQueryString(input)
			if remainder != "" || len(persisted.Clauses) != 1 {
				t.Fatalf("%s persisted as %+v, remainder %q", input, persisted, remainder)
			}
			search := &SearchPredicates{now: now}
			if input[1] == '@' {
				search.deferral = compileSearchDefer("2026-03-08", now)
			} else {
				search.due = compileSearchDue("2026-03-08", now)
			}
			for _, storedLoc := range []*time.Location{time.UTC, havana} {
				for _, day := range []int{7, 8, 9} {
					for _, hour := range []int{0, 12, 23} {
						stored := time.Date(2026, time.March, day, hour, 0, 0, 0, storedLoc)
						task := &models.Task{ID: "t", WorkflowState: models.StateBacklog, DueDate: &stored, DeferredUntil: &stored}
						if typed, saved := search.Admits(task, project), persisted.Admits(task, project, now); typed != saved {
							t.Errorf("%s in %s, stored %s: search admits %t, persisted query %t", input, loc, stored, typed, saved)
						}
					}
				}
			}
		}
	}
}
