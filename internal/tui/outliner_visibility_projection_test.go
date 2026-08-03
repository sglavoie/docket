package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
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
	betaSnoozed := primaryRowsTask("snoozed", models.StateBacklog)
	betaSnoozed.Pinned, betaSnoozed.PinnedAt, betaSnoozed.SnoozedUntil = true, &newest, &tomorrow
	betaDone := primaryRowsTask("done", models.StateDone)
	betaDone.Pinned, betaDone.PinnedAt = true, &newest
	beta := primaryRowsProject("beta", betaTie, betaSameID, betaSnoozed, betaDone)
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
	if !projection.projected[TaskIdentity{ProjectID: "alpha", TaskID: "shared"}] ||
		!projection.projected[TaskIdentity{ProjectID: "beta", TaskID: "shared"}] {
		t.Fatalf("qualified projected identities = %v, want both same-ID tasks for later-section deduplication", projection.projected)
	}
	for _, hidden := range []TaskIdentity{
		{ProjectID: "beta", TaskID: "snoozed"},
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

	sameDayTask := primaryRowsTask("same-day", models.StateBacklog)
	sameDayTask.Pinned = true
	sameDayTask.SnoozedUntil = &sameDay
	sameDayTask.DueDate = &sameDay
	snoozed := primaryRowsTask("snoozed", models.StateBacklog)
	snoozed.Pinned = true
	snoozed.SnoozedUntil = &nextDay
	done := primaryRowsTask("done", models.StateDone)
	done.Pinned = true
	project := primaryRowsProject("project", sameDayTask, snoozed, done)

	defaultProjection := buildPinnedModeProjection(OutlinerModeInput{
		Projects: []*models.Project{project},
		Now:      now,
	})
	if got := projectedTaskIDs(defaultProjection); !slices.Equal(got, []string{"same-day"}) {
		t.Fatalf("default projection = %v, want only open work whose snooze expires on the reference date", got)
	}

	explicitProjection := buildPinnedModeProjection(OutlinerModeInput{
		Projects:        []*models.Project{project},
		Now:             now,
		IncludeSnoozed:  true,
		IncludeTerminal: true,
	})
	if got := projectedTaskIDs(explicitProjection); !slices.Equal(got, []string{"same-day", "snoozed", "done"}) {
		t.Fatalf("explicit projection = %v, want snoozed and terminal opt-in", got)
	}

	if TaskMatchesSearch(sameDayTask, "@today") {
		t.Fatal("scope search interpreted due syntax instead of matching literal text")
	}
	sameDayTask.Description = "Discuss @today syntax"
	if !TaskMatchesSearch(sameDayTask, "@TODAY") {
		t.Fatal("scope search did not match case-insensitive description text")
	}

	common := CommonFilterState{DueDate: DueDateToday, Snoozed: true}
	if passesCommonFiltersAt(sameDayTask, &common, now) {
		t.Fatal("same-day task incorrectly matched the snoozed facet")
	}
	if !passesCommonFiltersAt(snoozed, &CommonFilterState{Snoozed: true}, now) {
		t.Fatal("future-date task did not match the snoozed facet at the shared reference time")
	}
}

func projectedTaskIDs(projection pinnedModeProjection) []string {
	ids := make([]string, 0, len(projection.rows))
	for _, row := range projection.rows {
		ids = append(ids, row.Task.ID)
	}
	return ids
}
