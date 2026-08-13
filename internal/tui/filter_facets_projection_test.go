package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

	outliner := scopeTestOutliner(work)
	states, err := models.NewWorkflowStateSet(models.StateBacklog, models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(states, true)
	outliner.filter.Priority = 3
	outliner.filter.Tags = map[string]bool{"alpha": true, "beta": true}
	outliner.filter.DueDate = DueDateToday
	outliner.filter.Deferred = true
	outliner.rebuildRows()

	visible := visibleOutlinerTaskIDs(outliner)
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
	outliner := scopeTestOutliner(project)
	outliner.filter.Tags = map[string]bool{"bug": true}
	outliner.rebuildRows()

	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{child.ID}) {
		t.Fatalf("#bug rows = %v, want only the tagged child", got)
	}
	row, ok := findPlanningTaskRow(outliner, child.ID)
	if !ok || row.Breadcrumb != parent.Title+" ›" {
		t.Fatalf("promoted child row = %+v found=%t, want parent breadcrumb", row, ok)
	}
	for _, row := range outliner.rows {
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
		mode  OutlinerModeID
		state models.WorkflowState
	}{
		{name: "inbox", mode: ModeInbox, state: models.StateInbox},
		{name: "backlog", mode: ModeBacklog, state: models.StateBacklog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := filterFacetTask("parent", tc.state, 0, []string{"other"}, nil, nil)
			parent.Title = "Parent"
			child := filterFacetTask("child", tc.state, 0, []string{"bug"}, nil, nil)
			child.ParentID = &parent.ID
			project := primaryRowsProject("project", parent, child)
			outliner := scopeTestOutliner(project)
			outliner.selectOutlinerMode(tc.mode)
			outliner.filter.Tags = map[string]bool{"bug": true}
			outliner.rebuildRows()

			if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{child.ID}) {
				t.Fatalf("%s #bug rows = %v, want only the tagged child", tc.mode, got)
			}
			row, ok := findPlanningTaskRow(outliner, child.ID)
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
	outliner := scopeTestOutliner(project)
	outliner.selectOutlinerMode(ModeBacklog)
	outliner.crossFocusStack = []CrossProjectFocusEntry{{ProjectID: project.ID, TaskID: focusRoot.ID}}
	outliner.filter.Tags = map[string]bool{"bug": true}
	outliner.rebuildRows()

	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{match.ID}) {
		t.Fatalf("focused #bug rows = %v, want only the tagged descendant", got)
	}
	row, ok := findPlanningTaskRow(outliner, match.ID)
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
			outliner := scopeTestOutliner(project)
			outliner.selectOutlinerMode(ModeBacklog)
			outliner.filter.Tags = map[string]bool{"bug": true}
			outliner.filter.Priority = 1
			outliner.rebuildRows()

			if got := visibleTaskIDs(outliner); !slices.Equal(got, tc.want) {
				t.Fatalf("Backlog filtered rows = %v, want %v", got, tc.want)
			}
			for _, row := range outliner.rows {
				if row.isTaskRow() && !passesCommonFilters(row.task, &outliner.filter.CommonFilterState) {
					t.Fatalf("visible context row %q does not satisfy active facets", row.task.ID)
				}
			}
			if tc.parentPriority > 1 {
				row, ok := findPlanningTaskRow(outliner, child.ID)
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
	outliner := scopeTestOutliner(project)
	states, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(states, true)
	outliner.rebuildRows()

	visible := visibleOutlinerTaskIDs(outliner)
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

	filter := newFilterState()
	filter.setCompletionAgeFilter(CompletionAge7Days)
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

	filter.setCompletionAgeFilter(CompletionAgeAll)
	if !passesCommonFiltersAt(tooRecent, &filter, now) {
		t.Fatal("all ages should remove the cutoff for a valid CompletedAt")
	}
	if passesCommonFiltersAt(missing, &filter, now) {
		t.Fatal("all ages should still exclude a missing CompletedAt")
	}
	if got := commonFilterParts(&filter); len(got) != 1 || got[0] != "completed:all" {
		t.Fatalf("completion-age badge = %v, want completed:all", got)
	}
}

func TestCompletionCleanupProjectionRequiresExplicitTerminalStateChoice(t *testing.T) {
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
	outliner := scopeTestOutliner(project)
	outliner.filter.setCompletionAgeFilter(CompletionAge30Days)
	outliner.rebuildRows()

	if visible := visibleOutlinerTaskIDs(outliner); len(visible) != 0 {
		t.Fatalf("fc without fs exposed cleanup records: %v", visible)
	}
	doneOnly, err := models.NewWorkflowStateSet(models.StateDone)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(doneOnly, true)
	outliner.rebuildRows()
	if visible := visibleOutlinerTaskIDs(outliner); !visible[doneOld.ID] || len(visible) != 1 {
		t.Fatalf("Done + 30-day cleanup projection = %v, want only old Done", visible)
	}
	both, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(both, true)
	outliner.rebuildRows()
	visible := visibleOutlinerTaskIDs(outliner)
	if !visible[doneOld.ID] || !visible[canceledOld.ID] || len(visible) != 2 {
		t.Fatalf("Done+Canceled + 30-day cleanup projection = %v, want both old terminal tasks", visible)
	}
}

func TestFilterFacetsPersistAcrossModeAndProjectScopeChanges(t *testing.T) {
	alpha := primaryRowsProject("alpha", filterFacetTask("alpha-task", models.StateBacklog, 1, []string{"alpha"}, nil, nil))
	beta := primaryRowsProject("beta", filterFacetTask("beta-task", models.StateWaiting, 2, []string{"beta"}, nil, nil))
	outliner := scopeTestOutliner(alpha, beta)

	states, err := models.NewWorkflowStateSet(models.StateBacklog, models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(states, true)
	outliner.filter.Priority = 2
	outliner.filter.Tags = map[string]bool{"alpha": true}
	outliner.filter.DueDate = DueDateNone
	outliner.filter.setCompletionAgeFilter(CompletionAge30Days)
	outliner.filter.Deferred = true

	outliner.selectOutlinerMode(ModeDue)
	if outliner.filter.Priority != 2 || !outliner.filter.Deferred {
		t.Fatalf("mode change reset facets: %+v", outliner.filter.CommonFilterState)
	}

	outliner.pushProjectScope(alpha.ID)
	if outliner.filter.Priority != 2 ||
		!outliner.filter.Tags["alpha"] ||
		!outliner.filter.States.Equal(states) ||
		outliner.filter.DueDate != DueDateNone ||
		!outliner.filter.hasCompletionAgeFilter() ||
		outliner.filter.CompletionAge != CompletionAge30Days ||
		!outliner.filter.Deferred {
		t.Fatalf("project scope entry reset facets: %+v", outliner.filter.CommonFilterState)
	}

	// Facet edits made inside one project scope belong to the session-wide
	// Outliner and must survive both focus replacement and the Esc-equivalent
	// pop back to the base scope.
	outliner.filter.Priority = models.PriorityMedium
	outliner.filter.Tags = map[string]bool{"beta": true}
	outliner.pushProjectScope(beta.ID)
	if outliner.filter.Priority != models.PriorityMedium || !outliner.filter.Tags["beta"] {
		t.Fatalf("project focus replacement reset facets: %+v", outliner.filter.CommonFilterState)
	}
	if !outliner.popProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if outliner.filter.Priority != models.PriorityMedium || !outliner.filter.Tags["beta"] {
		t.Fatalf("project scope pop restored stale facets: %+v", outliner.filter.CommonFilterState)
	}
}

func TestFilterFacetsAreExcludedFromPersistedRouteState(t *testing.T) {
	project := primaryRowsProject("project", filterFacetTask("task", models.StateBacklog, 1, nil, nil, nil))
	outliner := scopeTestOutliner(project)
	liveStates, err := models.NewWorkflowStateSet(models.StateBacklog)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(liveStates, true)
	outliner.filter.Priority = 2
	outliner.filter.Tags = map[string]bool{"live": true}
	outliner.filter.setCompletionAgeFilter(CompletionAge7Days)

	legacyStates, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.applyOutlinerRouteState(config.OutlinerRouteState{
		Mode:           config.OutlinerModeDue,
		FilterStates:   legacyStates,
		FilterDeferred: true,
		FilterPinned:   true,
	})
	if outliner.activeOutlinerMode() != ModeDue {
		t.Fatalf("route mode = %s, want Due", outliner.activeOutlinerMode())
	}
	if !outliner.filter.States.Equal(liveStates) ||
		outliner.filter.Priority != 2 ||
		!outliner.filter.Tags["live"] ||
		!outliner.filter.hasCompletionAgeFilter() ||
		outliner.filter.CompletionAge != CompletionAge7Days ||
		outliner.filter.Deferred ||
		outliner.filter.Pinned {
		t.Fatalf("legacy route restored session-only facets: %+v", outliner.filter.CommonFilterState)
	}

}
