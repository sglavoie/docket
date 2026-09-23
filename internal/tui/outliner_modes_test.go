package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
)

func TestOutlinerModeCatalogIsExactSevenModeCycle(t *testing.T) {
	want := []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent}
	catalog := chrome.OutlinerModeCatalog()
	tabModes := chrome.OutlinerTabModes()
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
	for _, retired := range []chrome.OutlinerModeID{
		"priority", "blocked", "parked", "daily-review", "activity",
	} {
		if _, ok := chrome.OutlinerModeByID(retired); ok {
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
	outlinerView := outliner.NewOutlinerModel(project, nil, nil)

	for _, want := range []chrome.OutlinerModeID{
		chrome.ModeToday, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent, chrome.ModeAll,
	} {
		outlinerView.CycleWorkMode(1)
		if got := outlinerView.ActiveOutlinerMode(); got != want {
			t.Fatalf("forward cycle mode = %q, want %q", got, want)
		}
	}
	for _, want := range []chrome.OutlinerModeID{
		chrome.ModeRecent, chrome.ModeDue, chrome.ModeBacklog, chrome.ModeNext, chrome.ModeInbox, chrome.ModeToday, chrome.ModeAll,
	} {
		outlinerView.CycleWorkMode(-1)
		if got := outlinerView.ActiveOutlinerMode(); got != want {
			t.Fatalf("reverse cycle mode = %q, want %q", got, want)
		}
	}
}

func TestEnteringRecentClearsOnlyStaleFacet(t *testing.T) {
	const status = "Stale filter cleared: Recent only includes the last 30 days"
	latest := recentTask("latest", 0)
	latest.Priority = models.PriorityMedium
	latest.Tags = []string{"deep"}
	tests := []struct {
		name             string
		tasks            []*models.Task
		enter            func(*outliner.OutlinerModel)
		wantEmpty        bool
		wantCursorTaskID string
	}{
		{
			name: "direct",
			enter: func(outlinerView *outliner.OutlinerModel) {
				outlinerView.SelectOutlinerMode(chrome.ModeRecent)
			},
		},
		{
			name: "Tab",
			enter: func(outlinerView *outliner.OutlinerModel) {
				outlinerView.CycleWorkMode(1)
			},
		},
		{
			name: "Ctrl+L empty",
			enter: func(outlinerView *outliner.OutlinerModel) {
				outlinerView.FocusLatestActivity()
			},
			wantEmpty: true,
		},
		{
			name:  "Ctrl+L successful jump",
			tasks: []*models.Task{latest},
			enter: func(outlinerView *outliner.OutlinerModel) {
				outlinerView.FocusLatestActivity()
			},
			wantCursorTaskID: latest.ID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := primaryRowsProject("alpha", test.tasks...)
			project.Tags = []string{"client"}
			outlinerView := outliner.NewOutlinerModel(project, nil, nil)
			outlinerView.SelectOutlinerMode(chrome.ModeDue)
			outlinerView.Filter.Priority = models.PriorityMedium
			outlinerView.Filter.Tags = map[string]bool{"deep": true}
			outlinerView.Filter.Stale = true
			outlinerView.ProjectFilter.Tags = map[string]bool{"client": true}
			outlinerView.ProjectFilter.ParkedMode = query.ProjectPopulationInclude
			adoptFilterCarriers(outlinerView)

			test.enter(outlinerView)

			if got := outlinerView.ActiveOutlinerMode(); got != chrome.ModeRecent {
				t.Fatalf("mode = %q, want %q", got, chrome.ModeRecent)
			}
			if outlinerView.Filter.Stale {
				t.Fatal("Recent retained the Stale facet")
			}
			if outlinerView.Filter.Priority != models.PriorityMedium || !outlinerView.Filter.Tags["deep"] {
				t.Fatalf("Recent changed unrelated common filters: %+v", outlinerView.Filter.CommonFilterState)
			}
			if !outlinerView.ProjectFilter.Tags["client"] ||
				outlinerView.ProjectFilter.ParkedMode != query.ProjectPopulationInclude {
				t.Fatalf("Recent changed project filters: %+v", outlinerView.ProjectFilter)
			}
			if outlinerView.StatusMessage != status || !outlinerView.StatusIsTransient() {
				t.Fatalf("status = %q transient=%t, want transient %q",
					outlinerView.StatusMessage, outlinerView.StatusIsTransient(), status)
			}
			if test.wantEmpty && len(visibleTaskIDs(outlinerView)) != 0 {
				t.Fatalf("Recent rows = %v, want an empty Ctrl+L result", visibleTaskIDs(outlinerView))
			}
			if test.wantCursorTaskID != "" && cursorTaskID(outlinerView) != test.wantCursorTaskID {
				t.Fatalf("cursor task = %q, want %q after Ctrl+L",
					cursorTaskID(outlinerView), test.wantCursorTaskID)
			}
		})
	}
}

func TestProjectScopePopClearsStaleWhenRestoringRecent(t *testing.T) {
	const status = "Stale filter cleared: Recent only includes the last 30 days"
	project := primaryRowsProject("alpha", recentTask("latest", 0))
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeRecent)

	// gp captures Recent as the base scope before switching the focused
	// project to All, where fs is available.
	outlinerView.PushProjectScope(project.ID)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	result := applyOutlinerFilterOperation(outlinerView, operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterStale},
	})
	if result.Err != nil {
		t.Fatalf("apply Stale filter in focused project: %v", result.Err)
	}
	if !outlinerView.Filter.Stale {
		t.Fatal("fs did not enable the Stale facet in All")
	}

	// Esc returns through popProjectScope to the captured Recent frame.
	if !outlinerView.PopProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if got := outlinerView.ActiveOutlinerMode(); got != chrome.ModeRecent {
		t.Fatalf("mode = %q, want %q", got, chrome.ModeRecent)
	}
	if outlinerView.Filter.Stale {
		t.Fatal("scope pop restored Recent with the Stale facet active")
	}
	if outlinerView.StatusMessage != status || !outlinerView.StatusIsTransient() {
		t.Fatalf("status = %q transient=%t, want transient %q",
			outlinerView.StatusMessage, outlinerView.StatusIsTransient(), status)
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

	outlinerView, _ := primaryRowsOutliner(alpha, beta)
	outlinerView.SelectOutlinerMode(chrome.ModeDue)
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{
		alphaOverdue.ID,
		betaOverdue.ID,
		alphaToday.ID,
		betaLater.ID,
	}) {
		t.Fatalf("global Due rows = %v, want dated open work in date order", got)
	}

	outlinerView.Scope.ProjectID = alpha.ID
	outlinerView.SelectOutlinerMode(chrome.ModeDue)
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{
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
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeDue)

	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{parent.ID, child.ID, other.ID}) {
		t.Fatalf("same-bucket Due rows = %v, want nested hierarchy", got)
	}
	parentRow, parentFound := findPlanningTaskRow(outlinerView, parent.ID)
	childRow, childFound := findPlanningTaskRow(outlinerView, child.ID)
	if !parentFound || !childFound || !parentRow.HasVisibleChildren || childRow.Breadcrumb != "" ||
		len(childRow.Prefix) <= len(parentRow.Prefix) {
		t.Fatalf("same-bucket Due hierarchy = parent %+v found=%t child %+v found=%t",
			parentRow, parentFound, childRow, childFound)
	}

	tomorrow := now.AddDate(0, 0, 1)
	child.DueDate = &tomorrow
	outlinerView.RebuildRows()
	childRow, childFound = findPlanningTaskRow(outlinerView, child.ID)
	if !childFound || childRow.Breadcrumb != parent.Title+" ›" {
		t.Fatalf("cross-bucket Due child = %+v found=%t, want ancestor breadcrumb", childRow, childFound)
	}
}

func TestBacklogOrdersSiblingsByPriorityThenManualOrder(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	firstSibling := primaryRowsTask("first-sibling", models.StateBacklog)
	firstSibling.Priority = models.PriorityMin
	secondSibling := primaryRowsTask("second-sibling", models.StateBacklog)
	secondSibling.Priority = models.PriorityMax
	thirdSibling := primaryRowsTask("third-sibling", models.StateBacklog)
	thirdSibling.Priority = models.PriorityMin
	project := primaryRowsProject("alpha", parent, child, firstSibling, secondSibling, thirdSibling)

	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{
		firstSibling.ID,
		thirdSibling.ID,
		secondSibling.ID,
		parent.ID,
		child.ID,
	}) {
		t.Fatalf("Backlog rows = %v, want priority order with manual order breaking ties", got)
	}
}
