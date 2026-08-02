package tui

import (
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
	snoozedUntil *time.Time,
) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         id,
		WorkflowState: state,
		Priority:      priority,
		Tags:          tags,
		DueDate:       dueDate,
		SnoozedUntil:  snoozedUntil,
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
		filterFacetTask("not-snoozed", models.StateBacklog, 2, commonTags, &today, nil),
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
	outliner.filter.Snoozed = true
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
		"not-snoozed",
	} {
		if visible[id] {
			t.Errorf("composed facets retained %q; visible = %v", id, visible)
		}
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
	outliner.filter.Blocked = true
	outliner.filter.Snoozed = true

	outliner.selectOutlinerMode(ModeDue)
	if outliner.filter.Priority != 2 || !outliner.filter.Blocked || !outliner.filter.Snoozed {
		t.Fatalf("mode change reset facets: %+v", outliner.filter.CommonFilterState)
	}

	outliner.pushProjectScope(alpha.ID)
	if outliner.filter.Priority != 2 ||
		!outliner.filter.Tags["alpha"] ||
		!outliner.filter.States.Equal(states) ||
		outliner.filter.DueDate != DueDateNone ||
		!outliner.filter.Blocked ||
		!outliner.filter.Snoozed {
		t.Fatalf("project scope entry reset facets: %+v", outliner.filter.CommonFilterState)
	}

	// Facet edits made inside one project scope belong to the session-wide
	// Outliner and must survive both focus replacement and the Esc-equivalent
	// pop back to the base scope.
	outliner.filter.Priority = models.PriorityMedium
	outliner.filter.Tags = map[string]bool{"beta": true}
	outliner.filter.Blocked = false
	outliner.pushProjectScope(beta.ID)
	if outliner.filter.Priority != models.PriorityMedium || !outliner.filter.Tags["beta"] || outliner.filter.Blocked {
		t.Fatalf("project focus replacement reset facets: %+v", outliner.filter.CommonFilterState)
	}
	if !outliner.popProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if outliner.filter.Priority != models.PriorityMedium || !outliner.filter.Tags["beta"] || outliner.filter.Blocked {
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
	outliner.filter.Blocked = true

	legacyStates, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.applyOutlinerRouteState(config.OutlinerRouteState{
		Mode:          config.OutlinerModeDue,
		FilterStates:  legacyStates,
		FilterSnoozed: true,
		FilterPinned:  true,
	})
	if outliner.activeOutlinerMode() != ModeDue {
		t.Fatalf("route mode = %s, want Due", outliner.activeOutlinerMode())
	}
	if !outliner.filter.States.Equal(liveStates) ||
		outliner.filter.Priority != 2 ||
		!outliner.filter.Tags["live"] ||
		!outliner.filter.Blocked ||
		outliner.filter.Snoozed ||
		outliner.filter.Pinned {
		t.Fatalf("legacy route restored session-only facets: %+v", outliner.filter.CommonFilterState)
	}

}
