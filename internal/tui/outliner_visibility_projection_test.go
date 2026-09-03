package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestPinnedModeProjectionIsNewestFirstAndProjectionOwned(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	newest := now.Add(-time.Hour)
	older := now.Add(-2 * time.Hour)
	tomorrow := now.AddDate(0, 0, 1)

	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("shared", models.StateBacklog)
	child.ParentID = &parent.ID
	child.Pinned, child.PinnedAt = true, &newest
	alphaTie := primaryRowsTask("alpha-tie", models.StateBacklog)
	alphaTie.Pinned, alphaTie.PinnedAt = true, &older
	alphaNoTimestamp := primaryRowsTask("alpha-no-time", models.StateBacklog)
	alphaNoTimestamp.Pinned = true
	alpha := primaryRowsProject("alpha", parent, child, alphaTie, alphaNoTimestamp)
	alpha.Name = "Alpha"

	betaTie := primaryRowsTask("beta-tie", models.StateBacklog)
	betaTie.Pinned, betaTie.PinnedAt = true, &older
	betaSameID := primaryRowsTask("shared", models.StateBacklog)
	betaSameID.Pinned, betaSameID.PinnedAt = true, &older
	betaDeferred := primaryRowsTask("deferred", models.StateBacklog)
	betaDeferred.Pinned, betaDeferred.PinnedAt, betaDeferred.DeferredUntil = true, &newest, &tomorrow
	betaDone := primaryRowsTask("done", models.StateDone)
	betaDone.Pinned, betaDone.PinnedAt = true, &newest
	beta := primaryRowsProject("beta", betaTie, betaSameID, betaDeferred, betaDone)
	beta.Name = "Beta"

	projection := buildPinnedModeProjection(OutlinerModeInput{
		Projects: []*models.Project{beta, alpha},
		Now:      now,
	})
	var got []string
	for _, row := range projection.rows {
		got = append(got, row.Project.ID+"/"+row.Task.ID)
	}
	want := []string{
		"alpha/shared",
		"beta/beta-tie",
		"beta/shared",
		"alpha/alpha-tie",
		"alpha/alpha-no-time",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pinned projection = %v, want newest-first deterministic order %v", got, want)
	}
	if projection.rows[0].Breadcrumb != "parent ›" {
		t.Fatalf("child breadcrumb = %q, want parent path", projection.rows[0].Breadcrumb)
	}
	if !projection.projected[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "shared"}] ||
		!projection.projected[taskref.TaskIdentity{ProjectID: "beta", TaskID: "shared"}] {
		t.Fatalf("qualified projected identities = %v, want both same-ID tasks for later-section deduplication", projection.projected)
	}
	for _, hidden := range []taskref.TaskIdentity{
		{ProjectID: "beta", TaskID: "deferred"},
		{ProjectID: "beta", TaskID: "done"},
	} {
		if projection.projected[hidden] {
			t.Fatalf("default projection included hidden task %v", hidden)
		}
	}
}

func TestVisibilityOverridesAndDateSensitiveFiltersShareReferenceTime(t *testing.T) {
	referenceLocation := time.FixedZone("reference", 9*60*60)
	dateLocation := time.FixedZone("date", -8*60*60)
	now := time.Date(2026, 3, 10, 0, 30, 0, 0, referenceLocation)
	sameDay := time.Date(2026, 3, 10, 0, 0, 0, 0, dateLocation)
	nextDay := time.Date(2026, 3, 11, 0, 0, 0, 0, dateLocation)
	// A deferral that expired earlier on the reference day is what schema 10
	// made representable, and it must project as ordinary open work.
	sameDayExpiry := time.Date(2026, 3, 10, 0, 0, 0, 0, referenceLocation)

	sameDayTask := primaryRowsTask("same-day", models.StateBacklog)
	sameDayTask.Pinned = true
	sameDayTask.DeferredUntil = &sameDayExpiry
	sameDayTask.DueDate = &sameDay
	deferred := primaryRowsTask("deferred", models.StateBacklog)
	deferred.Pinned = true
	deferred.DeferredUntil = &nextDay
	done := primaryRowsTask("done", models.StateDone)
	done.Pinned = true
	project := primaryRowsProject("project", sameDayTask, deferred, done)

	defaultProjection := buildPinnedModeProjection(OutlinerModeInput{
		Projects: []*models.Project{project},
		Now:      now,
	})
	if got := projectedTaskIDs(defaultProjection); !slices.Equal(got, []string{"same-day"}) {
		t.Fatalf("default projection = %v, want only open work whose deferral already expired at the reference time", got)
	}

	explicitProjection := buildPinnedModeProjection(OutlinerModeInput{
		Projects:        []*models.Project{project},
		Now:             now,
		IncludeDeferred: true,
		IncludeTerminal: true,
	})
	if got := projectedTaskIDs(explicitProjection); !slices.Equal(got, []string{"same-day", "deferred", "done"}) {
		t.Fatalf("explicit projection = %v, want deferred and terminal opt-in", got)
	}

	common := CommonFilterState{DueDate: DueDateToday, Deferred: true}
	if passesCommonFiltersAt(sameDayTask, &common, now) {
		t.Fatal("same-day task incorrectly matched the deferred facet")
	}
	if !passesCommonFiltersAt(deferred, &CommonFilterState{Deferred: true}, now) {
		t.Fatal("future-date task did not match the deferred facet at the shared reference time")
	}
}

func projectedTaskIDs(projection pinnedModeProjection) []string {
	ids := make([]string, 0, len(projection.rows))
	for _, row := range projection.rows {
		ids = append(ids, row.Task.ID)
	}
	return ids
}
