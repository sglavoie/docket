package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerModeCatalogIsExactSevenModeCycle(t *testing.T) {
	want := []OutlinerModeID{ModeAll, ModeToday, ModeInbox, ModeNext, ModeBacklog, ModeDue, ModeRecent}
	catalog := OutlinerModeCatalog()
	tabModes := OutlinerTabModes()
	if len(catalog) != len(want) {
		t.Fatalf("mode catalog length = %d, want %d", len(catalog), len(want))
	}
	if len(tabModes) != len(want) {
		t.Fatalf("Tab cycle length = %d, want %d", len(tabModes), len(want))
	}
	for index, mode := range catalog {
		if mode.ID != want[index] {
			t.Errorf("catalog[%d] = %q, want %q", index, mode.ID, want[index])
		}
		if tabModes[index].ID != mode.ID {
			t.Errorf("Tab cycle[%d] = %q, want catalog mode %q",
				index, tabModes[index].ID, mode.ID)
		}
	}
	for _, retired := range []OutlinerModeID{
		"priority", "blocked", "parked", "daily-review", "activity",
	} {
		if _, ok := OutlinerModeByID(retired); ok {
			t.Errorf("retired mode %q remains selectable", retired)
		}
	}

	for _, binding := range SimplifiedInputStrategy().Bindings {
		if slices.Contains(binding.Keys, "m") {
			t.Errorf("dedicated mode-picker key remains bound to %s", binding.OperationID)
		}
	}
}

func TestOutlinerTabCyclingWrapsForwardAndBackwardAcrossSevenModes(t *testing.T) {
	project := primaryRowsProject("alpha")
	outliner := NewOutlinerModel(project, nil, nil)

	for _, want := range []OutlinerModeID{
		ModeToday, ModeInbox, ModeNext, ModeBacklog, ModeDue, ModeRecent, ModeAll,
	} {
		outliner.cycleWorkMode(1)
		if got := outliner.activeOutlinerMode(); got != want {
			t.Fatalf("forward cycle mode = %q, want %q", got, want)
		}
	}
	for _, want := range []OutlinerModeID{
		ModeRecent, ModeDue, ModeBacklog, ModeNext, ModeInbox, ModeToday, ModeAll,
	} {
		outliner.cycleWorkMode(-1)
		if got := outliner.activeOutlinerMode(); got != want {
			t.Fatalf("reverse cycle mode = %q, want %q", got, want)
		}
	}
}

func TestEnteringRecentClearsOnlyStaleFacet(t *testing.T) {
	const status = "Stale filter cleared: Recent only includes the last 30 days"
	latest := recentTask("latest", models.StateBacklog, 0)
	latest.Priority = models.PriorityMedium
	latest.Tags = []string{"deep"}
	tests := []struct {
		name             string
		tasks            []*models.Task
		enter            func(*OutlinerModel)
		wantEmpty        bool
		wantCursorTaskID string
	}{
		{
			name: "direct",
			enter: func(outliner *OutlinerModel) {
				outliner.selectOutlinerMode(ModeRecent)
			},
		},
		{
			name: "Tab",
			enter: func(outliner *OutlinerModel) {
				outliner.cycleWorkMode(1)
			},
		},
		{
			name: "Ctrl+L empty",
			enter: func(outliner *OutlinerModel) {
				outliner.focusLatestActivity()
			},
			wantEmpty: true,
		},
		{
			name:  "Ctrl+L successful jump",
			tasks: []*models.Task{latest},
			enter: func(outliner *OutlinerModel) {
				outliner.focusLatestActivity()
			},
			wantCursorTaskID: latest.ID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := primaryRowsProject("alpha", test.tasks...)
			project.Tags = []string{"client"}
			outliner := NewOutlinerModel(project, nil, nil)
			outliner.selectOutlinerMode(ModeDue)
			outliner.filter.Priority = models.PriorityMedium
			outliner.filter.Tags = map[string]bool{"deep": true}
			outliner.filter.Stale = true
			outliner.projectFilter.Tags = map[string]bool{"client": true}
			outliner.projectFilter.ParkedMode = projectPopulationInclude

			test.enter(outliner)

			if got := outliner.activeOutlinerMode(); got != ModeRecent {
				t.Fatalf("mode = %q, want %q", got, ModeRecent)
			}
			if outliner.filter.Stale {
				t.Fatal("Recent retained the Stale facet")
			}
			if outliner.filter.Priority != models.PriorityMedium || !outliner.filter.Tags["deep"] {
				t.Fatalf("Recent changed unrelated common filters: %+v", outliner.filter.CommonFilterState)
			}
			if !outliner.projectFilter.Tags["client"] ||
				outliner.projectFilter.ParkedMode != projectPopulationInclude {
				t.Fatalf("Recent changed project filters: %+v", outliner.projectFilter)
			}
			if outliner.statusMessage != status || !outliner.StatusIsTransient() {
				t.Fatalf("status = %q transient=%t, want transient %q",
					outliner.statusMessage, outliner.StatusIsTransient(), status)
			}
			if test.wantEmpty && len(visibleTaskIDs(outliner)) != 0 {
				t.Fatalf("Recent rows = %v, want an empty Ctrl+L result", visibleTaskIDs(outliner))
			}
			if test.wantCursorTaskID != "" && cursorTaskID(outliner) != test.wantCursorTaskID {
				t.Fatalf("cursor task = %q, want %q after Ctrl+L",
					cursorTaskID(outliner), test.wantCursorTaskID)
			}
		})
	}
}

func TestProjectScopePopClearsStaleWhenRestoringRecent(t *testing.T) {
	const status = "Stale filter cleared: Recent only includes the last 30 days"
	project := primaryRowsProject("alpha", recentTask("latest", models.StateBacklog, 0))
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeRecent)

	// gp captures Recent as the base scope before switching the focused
	// project to All, where fs is available.
	outliner.pushProjectScope(project.ID)
	outliner.selectOutlinerMode(ModeAll)
	result := (&AppModel{crossOutliner: outliner}).executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterStale},
	})
	if result.Err != nil {
		t.Fatalf("apply Stale filter in focused project: %v", result.Err)
	}
	if !outliner.filter.Stale {
		t.Fatal("fs did not enable the Stale facet in All")
	}

	// Esc returns through popProjectScope to the captured Recent frame.
	if !outliner.popProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if got := outliner.activeOutlinerMode(); got != ModeRecent {
		t.Fatalf("mode = %q, want %q", got, ModeRecent)
	}
	if outliner.filter.Stale {
		t.Fatal("scope pop restored Recent with the Stale facet active")
	}
	if outliner.statusMessage != status || !outliner.StatusIsTransient() {
		t.Fatalf("status = %q transient=%t, want transient %q",
			outliner.statusMessage, outliner.StatusIsTransient(), status)
	}
}

func TestDueModeProjectsDatedOpenWorkInGlobalAndProjectScopes(t *testing.T) {
	now := time.Now()
	dueTask := func(id string, dayOffset int) *models.Task {
		task := primaryRowsTask(id, models.StateBacklog)
		due := now.AddDate(0, 0, dayOffset)
		task.DueDate = &due
		return task
	}

	alphaOverdue := dueTask("alpha-overdue", -2)
	alphaToday := dueTask("alpha-today", 0)
	alphaUndated := primaryRowsTask("alpha-undated", models.StateBacklog)
	alphaDone := dueTask("alpha-done", -3)
	alphaDone.WorkflowState = models.StateDone
	alpha := primaryRowsProject("alpha", alphaOverdue, alphaToday, alphaUndated, alphaDone)

	betaOverdue := dueTask("beta-overdue", -1)
	betaLater := dueTask("beta-later", 8)
	betaDeferred := dueTask("beta-deferred", 1)
	deferUntil := now.AddDate(0, 0, 2)
	betaDeferred.DeferredUntil = &deferUntil
	beta := primaryRowsProject("beta", betaOverdue, betaLater, betaDeferred)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeDue)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaOverdue.ID,
		betaOverdue.ID,
		alphaToday.ID,
		betaLater.ID,
	}) {
		t.Fatalf("global Due rows = %v, want dated open work in date order", got)
	}

	outliner.scopedProjectID = alpha.ID
	outliner.selectOutlinerMode(ModeDue)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaOverdue.ID,
		alphaToday.ID,
	}) {
		t.Fatalf("project-scoped Due rows = %v, want alpha dated open work", got)
	}
}

func TestDueModeNestsRelativesOnlyWithinSameDateBucket(t *testing.T) {
	now := time.Now()
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	other := primaryRowsTask("other", models.StateBacklog)
	parent.DueDate, child.DueDate, other.DueDate = &now, &now, &now
	project := primaryRowsProject("alpha", parent, child, other)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeDue)

	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{parent.ID, child.ID, other.ID}) {
		t.Fatalf("same-bucket Due rows = %v, want nested hierarchy", got)
	}
	parentRow, parentFound := findPlanningTaskRow(outliner, parent.ID)
	childRow, childFound := findPlanningTaskRow(outliner, child.ID)
	if !parentFound || !childFound || !parentRow.hasVisibleChildren || childRow.Breadcrumb != "" ||
		len(childRow.prefix) <= len(parentRow.prefix) {
		t.Fatalf("same-bucket Due hierarchy = parent %+v found=%t child %+v found=%t",
			parentRow, parentFound, childRow, childFound)
	}

	tomorrow := now.AddDate(0, 0, 1)
	child.DueDate = &tomorrow
	outliner.rebuildRows()
	childRow, childFound = findPlanningTaskRow(outliner, child.ID)
	if !childFound || childRow.Breadcrumb != parent.Title+" ›" {
		t.Fatalf("cross-bucket Due child = %+v found=%t, want ancestor breadcrumb", childRow, childFound)
	}
}

func TestBacklogRetainsProjectHierarchyAndManualSiblingOrder(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	firstSibling := primaryRowsTask("first-sibling", models.StateBacklog)
	firstSibling.Priority = models.PriorityMin
	secondSibling := primaryRowsTask("second-sibling", models.StateBacklog)
	secondSibling.Priority = models.PriorityMax
	project := primaryRowsProject("alpha", parent, child, firstSibling, secondSibling)

	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeBacklog)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		parent.ID,
		child.ID,
		firstSibling.ID,
		secondSibling.ID,
	}) {
		t.Fatalf("Backlog rows = %v, want manual hierarchy order", got)
	}
}
