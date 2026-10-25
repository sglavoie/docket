package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
)

func filterFacetTask(
	id string,
	state models.WorkflowState,
	priority int,
	tags []string,
	dueDate *time.Time,
	deferredUntil *time.Time,
) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         id,
		WorkflowState: state,
		Priority:      priority,
		Tags:          tags,
		DueDate:       dueDate,
		DeferredUntil: deferredUntil,
	}
}

func TestFilterFacetsComposeInCanonicalProjection(t *testing.T) {
	now := time.Now()
	today := now
	tomorrow := now.AddDate(0, 0, 1)

	commonTags := []string{"alpha", "beta"}
	work := primaryRowsProject(
		"work",
		filterFacetTask("ready-match", models.StateBacklog, 2, commonTags, &today, &tomorrow),
		filterFacetTask("waiting-match", models.StateWaiting, 3, commonTags, &today, &tomorrow),
		filterFacetTask("wrong-state", models.StateDoing, 2, commonTags, &today, &tomorrow),
		filterFacetTask("wrong-priority", models.StateBacklog, 5, commonTags, &today, &tomorrow),
		filterFacetTask("missing-tag", models.StateBacklog, 2, []string{"alpha"}, &today, &tomorrow),
		filterFacetTask("wrong-due", models.StateBacklog, 2, commonTags, &tomorrow, &tomorrow),
		filterFacetTask("not-deferred", models.StateBacklog, 2, commonTags, &today, nil),
	)

	outlinerView := scopeTestOutliner(work)
	states, err := models.NewWorkflowStateSet(models.StateBacklog, models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(states)
	outlinerView.Filter.Priority = 3
	outlinerView.Filter.Tags = map[string]bool{"alpha": true, "beta": true}
	outlinerView.Filter.DueDate = query.DueDateToday
	outlinerView.Filter.Deferred = true
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()

	visible := visibleOutlinerTaskIDs(outlinerView)
	for _, id := range []string{"ready-match", "waiting-match"} {
		if !visible[id] {
			t.Errorf("composed facets hid %q; visible = %v", id, visible)
		}
	}
	for _, id := range []string{
		"wrong-state",
		"wrong-priority",
		"missing-tag",
		"wrong-due",
		"not-deferred",
	} {
		if visible[id] {
			t.Errorf("composed facets retained %q; visible = %v", id, visible)
		}
	}
}

func TestTagFacetDropsNonmatchingAncestorAcrossWorkflowSections(t *testing.T) {
	parent := filterFacetTask(
		"curator-app",
		models.StateBacklog,
		0,
		[]string{"stale", "wip-old", "epic"},
		nil,
		nil,
	)
	parent.Title = "Curator app"
	child := filterFacetTask(
		"curator-crop-bug",
		models.StatePaused,
		0,
		[]string{"aging", "bug"},
		nil,
		nil,
	)
	child.ParentID = &parent.ID
	project := primaryRowsProject("life-trail", parent, child)
	outlinerView := scopeTestOutliner(project)
	outlinerView.Filter.Tags = map[string]bool{"bug": true}
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()

	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{child.ID}) {
		t.Fatalf("#bug rows = %v, want only the tagged child", got)
	}
	row, ok := findPlanningTaskRow(outlinerView, child.ID)
	if !ok || row.Breadcrumb != parent.Title+" ›" {
		t.Fatalf("promoted child row = %+v found=%t, want parent breadcrumb", row, ok)
	}
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader && row.StateGroup == models.StateGroupBacklog {
			t.Fatalf("untagged parent left a Backlog section behind: %+v", row)
		}
		if row.IsSectionHeader && row.StateGroup == models.StateGroupPaused && row.SectionCount != 1 {
			t.Fatalf("Paused section count = %d, want 1", row.SectionCount)
		}
	}
}

func TestHierarchicalModesPromoteMatchingChildPastFacetMismatch(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mode  chrome.OutlinerModeID
		state models.WorkflowState
	}{
		{name: "inbox", mode: chrome.ModeInbox, state: models.StateInbox},
		{name: "backlog", mode: chrome.ModeBacklog, state: models.StateBacklog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := filterFacetTask("parent", tc.state, 0, []string{"other"}, nil, nil)
			parent.Title = "Parent"
			child := filterFacetTask("child", tc.state, 0, []string{"bug"}, nil, nil)
			child.ParentID = &parent.ID
			project := primaryRowsProject("project", parent, child)
			outlinerView := scopeTestOutliner(project)
			outlinerView.SelectOutlinerMode(tc.mode)
			outlinerView.Filter.Tags = map[string]bool{"bug": true}
			adoptFilterCarriers(outlinerView)
			outlinerView.RebuildRows()

			if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{child.ID}) {
				t.Fatalf("%s #bug rows = %v, want only the tagged child", tc.mode, got)
			}
			row, ok := findPlanningTaskRow(outlinerView, child.ID)
			if !ok || row.Breadcrumb != parent.Title+" ›" {
				t.Fatalf("%s promoted child row = %+v found=%t, want parent breadcrumb", tc.mode, row, ok)
			}
		})
	}
}

func TestPromotedFacetBreadcrumbStopsAtFocusBoundary(t *testing.T) {
	focusRoot := filterFacetTask("focus-root", models.StateBacklog, 0, []string{"other"}, nil, nil)
	focusRoot.Title = "Focus root"
	hiddenParent := filterFacetTask("hidden-parent", models.StateBacklog, 0, []string{"other"}, nil, nil)
	hiddenParent.Title = "Hidden parent"
	hiddenParent.ParentID = &focusRoot.ID
	match := filterFacetTask("match", models.StateBacklog, 0, []string{"bug"}, nil, nil)
	match.ParentID = &hiddenParent.ID
	project := primaryRowsProject("project", focusRoot, hiddenParent, match)
	outlinerView := scopeTestOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
	outlinerView.Focus.CrossStack = []outliner.CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: focusRoot.ID}}
	outlinerView.Filter.Tags = map[string]bool{"bug": true}
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()

	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{match.ID}) {
		t.Fatalf("focused #bug rows = %v, want only the tagged descendant", got)
	}
	row, ok := findPlanningTaskRow(outlinerView, match.ID)
	if !ok || row.Breadcrumb != hiddenParent.Title+" ›" {
		t.Fatalf("focused promoted row = %+v found=%t, want breadcrumb below focus boundary", row, ok)
	}
}

func TestBacklogContextRowsMustSatisfyEveryActiveFacet(t *testing.T) {
	for _, tc := range []struct {
		name           string
		parentPriority int
		want           []string
	}{
		{name: "matching context remains", parentPriority: 1, want: []string{"parent", "child"}},
		{name: "nonmatching context is removed", parentPriority: 2, want: []string{"child"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := filterFacetTask("parent", models.StateDoing, tc.parentPriority, []string{"bug"}, nil, nil)
			parent.Title = "Parent"
			child := filterFacetTask("child", models.StateBacklog, 1, []string{"bug"}, nil, nil)
			child.ParentID = &parent.ID
			project := primaryRowsProject("project", parent, child)
			outlinerView := scopeTestOutliner(project)
			outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
			outlinerView.Filter.Tags = map[string]bool{"bug": true}
			outlinerView.Filter.Priority = 1
			adoptFilterCarriers(outlinerView)
			outlinerView.RebuildRows()

			if got := visibleTaskIDs(outlinerView); !slices.Equal(got, tc.want) {
				t.Fatalf("Backlog filtered rows = %v, want %v", got, tc.want)
			}
			for _, row := range outlinerView.Rows {
				if row.IsTaskRow() && !passesCommonFilters(row.Task, &outlinerView.Filter.CommonFilterState) {
					t.Fatalf("visible context row %q does not satisfy active facets", row.Task.ID)
				}
			}
			if tc.parentPriority > 1 {
				row, ok := findPlanningTaskRow(outlinerView, child.ID)
				if !ok || row.Breadcrumb != parent.Title+" ›" {
					t.Fatalf("promoted Backlog child row = %+v found=%t, want parent breadcrumb", row, ok)
				}
			}
		})
	}
}

func TestExplicitTerminalStateFacetOverridesDefaultVisibility(t *testing.T) {
	project := primaryRowsProject(
		"project",
		filterFacetTask("ready", models.StateBacklog, 0, nil, nil, nil),
		filterFacetTask("done", models.StateDone, 0, nil, nil, nil),
		filterFacetTask("canceled", models.StateCanceled, 0, nil, nil, nil),
	)
	outlinerView := scopeTestOutliner(project)
	states, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(states)
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()

	visible := visibleOutlinerTaskIDs(outlinerView)
	if !visible["done"] || !visible["canceled"] || visible["ready"] {
		t.Fatalf("terminal state OR facet produced %v, want done+canceled only", visible)
	}
}

func TestCompletionAgeFacetUsesLocalCalendarDaysAndRejectsInvalidTimestamps(t *testing.T) {
	location := time.FixedZone("cleanup-local", -6*60*60)
	now := time.Date(2026, time.August, 3, 0, 15, 0, 0, location)
	sevenLocalDaysAgo := time.Date(2026, time.July, 27, 23, 59, 0, 0, location)
	justSixLocalDaysAgoUTC := time.Date(2026, time.July, 28, 12, 30, 0, 0, time.UTC)
	zero := time.Time{}

	filter := query.NewCommonFilterState()
	filter.SetCompletionAgeFilter(query.CompletionAge7Days)
	oldEnough := &models.Task{ID: "old", WorkflowState: models.StateDone, CompletedAt: &sevenLocalDaysAgo}
	tooRecent := &models.Task{ID: "recent", WorkflowState: models.StateDone, CompletedAt: &justSixLocalDaysAgoUTC}
	missing := &models.Task{ID: "missing", WorkflowState: models.StateDone}
	zeroTimestamp := &models.Task{ID: "zero", WorkflowState: models.StateCanceled, CompletedAt: &zero}

	if !passesCommonFiltersAt(oldEnough, &filter, now) {
		t.Fatal("completion on the seventh local calendar day should match")
	}
	if passesCommonFiltersAt(tooRecent, &filter, now) {
		t.Fatal("UTC timestamp on the sixth local calendar day should not match")
	}
	if passesCommonFiltersAt(missing, &filter, now) || passesCommonFiltersAt(zeroTimestamp, &filter, now) {
		t.Fatal("cleanup facet matched a task without a valid CompletedAt")
	}

	filter.SetCompletionAgeFilter(query.CompletionAgeAll)
	if !passesCommonFiltersAt(tooRecent, &filter, now) {
		t.Fatal("all ages should remove the cutoff for a valid CompletedAt")
	}
	if passesCommonFiltersAt(missing, &filter, now) {
		t.Fatal("all ages should still exclude a missing CompletedAt")
	}
	if got := query.CommonFilterParts(&filter); len(got) != 1 || got[0] != "?completed" {
		t.Fatalf("completion-age badge = %v, want ?completed", got)
	}
}

func TestStaleFacetUsesUpdatedLocalCalendarAgeAndBaseVisibility(t *testing.T) {
	location := time.FixedZone("stale-local", -6*60*60)
	now := time.Date(2026, time.August, 3, 0, 15, 0, 0, location)
	day90 := time.Date(2026, time.May, 5, 23, 59, 0, 0, location)
	day89 := time.Date(2026, time.May, 6, 23, 59, 0, 0, location)
	wake := now.AddDate(0, 0, 1)

	filter := query.NewCommonFilterState()
	filter.Stale = true
	oldEnough := &models.Task{ID: "old", WorkflowState: models.StateBacklog, UpdatedAt: day90}
	tooRecent := &models.Task{ID: "recent", WorkflowState: models.StateBacklog, UpdatedAt: day89}
	terminal := &models.Task{ID: "done", WorkflowState: models.StateDone, UpdatedAt: day90}
	deferred := &models.Task{ID: "deferred", WorkflowState: models.StateBacklog, UpdatedAt: day90, DeferredUntil: &wake}

	if !passesCommonFiltersAt(oldEnough, &filter, now) {
		t.Fatal("touch on the ninetieth local calendar day should match Stale")
	}
	if passesCommonFiltersAt(tooRecent, &filter, now) {
		t.Fatal("touch on the eighty-ninth local calendar day should not match Stale")
	}
	if passesCommonFiltersAt(terminal, &filter, now) || passesCommonFiltersAt(deferred, &filter, now) {
		t.Fatal("Stale facet admitted terminal or currently deferred work")
	}

	if !query.HasActiveCommonFilters(&filter) || query.CommonFilterFacetCount(&filter) != 1 {
		t.Fatalf("Stale is not the one active facet: %+v", filter)
	}
	clearCommonFilters(&filter)
	if filter.Stale || query.HasActiveCommonFilters(&filter) {
		t.Fatalf("clearCommonFilters left Stale active: %+v", filter)
	}
	if fresh := query.NewCommonFilterState(); fresh.Stale {
		t.Fatal("NewCommonFilterState starts with Stale active")
	}
}

// TestCompletionCleanupProjectionOpensOnTheCompletedClause is D3 read on the
// completion axis: a positive completed clause is the query saying so, so it
// opens the terminal population by itself, and adding a state clause narrows
// what that population shows rather than being what unlocks it.
func TestCompletionCleanupProjectionOpensOnTheCompletedClause(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -45)
	recent := now.AddDate(0, 0, -5)
	doneOld := filterFacetTask("done-old", models.StateDone, 0, nil, nil, nil)
	doneOld.CompletedAt = &old
	doneRecent := filterFacetTask("done-recent", models.StateDone, 0, nil, nil, nil)
	doneRecent.CompletedAt = &recent
	canceledOld := filterFacetTask("canceled-old", models.StateCanceled, 0, nil, nil, nil)
	canceledOld.CompletedAt = &old
	invalidDone := filterFacetTask("done-invalid", models.StateDone, 0, nil, nil, nil)
	ready := filterFacetTask("ready", models.StateBacklog, 0, nil, nil, nil)
	project := primaryRowsProject("cleanup", doneOld, doneRecent, canceledOld, invalidDone, ready)
	outlinerView := scopeTestOutliner(project)
	outlinerView.Filter.SetCompletionAgeFilter(query.CompletionAge30Days)
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()

	visible := visibleOutlinerTaskIDs(outlinerView)
	if !visible[doneOld.ID] || !visible[canceledOld.ID] || len(visible) != 2 {
		t.Fatalf("the completion-age clause alone = %v, want both old terminal records", visible)
	}
	doneOnly, err := models.NewWorkflowStateSet(models.StateDone)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(doneOnly)
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()
	visible = visibleOutlinerTaskIDs(outlinerView)
	if !visible[doneOld.ID] || len(visible) != 1 {
		t.Fatalf("Done + 30-day cleanup projection = %v, want only old Done", visible)
	}
	both, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(both)
	adoptFilterCarriers(outlinerView)
	outlinerView.RebuildRows()
	visible = visibleOutlinerTaskIDs(outlinerView)
	if !visible[doneOld.ID] || !visible[canceledOld.ID] || len(visible) != 2 {
		t.Fatalf("Done+Canceled + 30-day cleanup projection = %v, want both old terminal tasks", visible)
	}
}

func TestFilterFacetsPersistAcrossModeAndProjectScopeChanges(t *testing.T) {
	alpha := primaryRowsProject("alpha", filterFacetTask("alpha-task", models.StateBacklog, 1, []string{"alpha"}, nil, nil))
	beta := primaryRowsProject("beta", filterFacetTask("beta-task", models.StateWaiting, 2, []string{"beta"}, nil, nil))
	outlinerView := scopeTestOutliner(alpha, beta)

	states, err := models.NewWorkflowStateSet(models.StateBacklog, models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(states)
	outlinerView.Filter.Priority = 2
	outlinerView.Filter.Tags = map[string]bool{"alpha": true}
	outlinerView.Filter.DueDate = query.DueDateNone
	outlinerView.Filter.SetCompletionAgeFilter(query.CompletionAge30Days)
	outlinerView.Filter.Deferred = true
	adoptFilterCarriers(outlinerView)

	outlinerView.SelectOutlinerMode(chrome.ModeDue)
	if outlinerView.Filter.Priority != 2 || !outlinerView.Filter.Deferred {
		t.Fatalf("mode change reset facets: %+v", outlinerView.Filter.CommonFilterState)
	}

	outlinerView.PushProjectScope(alpha.ID)
	if outlinerView.Filter.Priority != 2 ||
		!outlinerView.Filter.Tags["alpha"] ||
		!outlinerView.Filter.States.Equal(states) ||
		outlinerView.Filter.DueDate != query.DueDateNone ||
		!outlinerView.Filter.HasCompletionAgeFilter() ||
		outlinerView.Filter.CompletionAge != query.CompletionAge30Days ||
		!outlinerView.Filter.Deferred {
		t.Fatalf("project scope entry reset facets: %+v", outlinerView.Filter.CommonFilterState)
	}

	// Facet edits made inside one project scope belong to the session-wide
	// Outliner and must survive both focus replacement and the Esc-equivalent
	// pop back to the base scope.
	outlinerView.Filter.Priority = models.PriorityMedium
	outlinerView.Filter.Tags = map[string]bool{"beta": true}
	adoptFilterCarriers(outlinerView)
	outlinerView.PushProjectScope(beta.ID)
	if outlinerView.Filter.Priority != models.PriorityMedium || !outlinerView.Filter.Tags["beta"] {
		t.Fatalf("project focus replacement reset facets: %+v", outlinerView.Filter.CommonFilterState)
	}
	if !outlinerView.PopProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if outlinerView.Filter.Priority != models.PriorityMedium || !outlinerView.Filter.Tags["beta"] {
		t.Fatalf("project scope pop restored stale facets: %+v", outlinerView.Filter.CommonFilterState)
	}
}

func TestFilterFacetsAreExcludedFromPersistedRouteState(t *testing.T) {
	project := primaryRowsProject("project", filterFacetTask("task", models.StateBacklog, 1, nil, nil, nil))
	outlinerView := scopeTestOutliner(project)
	liveStates, err := models.NewWorkflowStateSet(models.StateBacklog)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.Filter.SetStateFilter(liveStates)
	outlinerView.Filter.Priority = 2
	outlinerView.Filter.Tags = map[string]bool{"live": true}
	outlinerView.Filter.SetCompletionAgeFilter(query.CompletionAge7Days)
	adoptFilterCarriers(outlinerView)

	legacyStates, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outlinerView.ApplyOutlinerRouteState(config.OutlinerRouteState{
		Mode:           config.OutlinerModeDue,
		FilterStates:   legacyStates,
		FilterDeferred: true,
		FilterPinned:   true,
	})
	if outlinerView.ActiveOutlinerMode() != chrome.ModeDue {
		t.Fatalf("route mode = %s, want Due", outlinerView.ActiveOutlinerMode())
	}
	if !outlinerView.Filter.States.Equal(liveStates) ||
		outlinerView.Filter.Priority != 2 ||
		!outlinerView.Filter.Tags["live"] ||
		!outlinerView.Filter.HasCompletionAgeFilter() ||
		outlinerView.Filter.CompletionAge != query.CompletionAge7Days ||
		outlinerView.Filter.Deferred ||
		outlinerView.Filter.Pinned {
		t.Fatalf("legacy route restored session-only facets: %+v", outlinerView.Filter.CommonFilterState)
	}

}
