package tui

import (
	"image/color"
	"testing"
)

// The legacy* oracles below are verbatim copies of the pre-refactor trait
// switches and accent map. The parity tests assert that the declarative route
// table reproduces their behavior for every ViewType, so the refactor is proven
// behavior-preserving and future drift in the table is caught.

func legacyIsTrackedView(vt ViewType) bool {
	switch vt {
	case ViewDashboard,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewProjectFocusView,
		ViewPriorityView,
		ViewRecentActivity,
		ViewGlobalSearch,
		ViewGlobalBlockers,
		ViewGlobalNotes,
		ViewGlobalInboxProcessor,
		ViewGlobalTagManager,
		ViewNotes,
		ViewBlockers,
		ViewInboxProcessor,
		ViewSettings,
		ViewWorkflowRules,
		ViewTaskTemplates,
		ViewTutor,
		ViewScoreProfileManager,
		ViewTaskComments,
		ViewTaskDecisions,
		ViewTaskChecklist,
		ViewTaskDetail,
		ViewTaskAttachments,
		ViewTaskActivity,
		ViewMetricsDashboard,
		ViewAllTasks,
		ViewOutliner,
		ViewCrossProjectOutliner,
		ViewTemporaryOutliner,
		ViewFocusBoard,
		ViewProjectRecentActivity,
		ViewRoadmap:
		return true
	default:
		return false
	}
}

func legacyIsGlobalNavView(vt ViewType) bool {
	switch vt {
	case ViewDailyReview,
		ViewWeeklyReview,
		ViewRecentActivity,
		ViewGlobalSearch,
		ViewGlobalBlockers,
		ViewGlobalNotes,
		ViewGlobalInboxProcessor,
		ViewGlobalTagManager,
		ViewSettings,
		ViewWorkflowRules,
		ViewTaskTemplates,
		ViewTutor,
		ViewScoreProfileManager,
		ViewMetricsDashboard,
		ViewCrossProjectOutliner,
		ViewFocusBoard,
		ViewAllTasks,
		ViewRoadmap:
		return true
	default:
		return false
	}
}

func legacyIsTaskScopedView(vt ViewType) bool {
	switch vt {
	case ViewTaskComments, ViewTaskDecisions, ViewTaskChecklist, ViewTaskDetail, ViewTaskAttachments, ViewTaskActivity:
		return true
	default:
		return false
	}
}

func legacyIsGlobalOverlayView(v ViewType) bool {
	switch v {
	case ViewFocusBoard, ViewGlobalInboxProcessor,
		ViewGlobalNotes, ViewRecentActivity, ViewGlobalBlockers,
		ViewMetricsDashboard, ViewAllTasks, ViewWorkflowRules,
		ViewPipelineHub, ViewRoadmap:
		return true
	default:
	}
	return false
}

func legacyAccentMap() map[ViewType]color.Color {
	return map[ViewType]color.Color{
		ViewDashboard:            AccentDashboard,
		ViewQuickCapture:         AccentCapture,
		ViewBulkCapture:          AccentCapture,
		ViewAllTasks:             AccentTriage,
		ViewGlobalInboxProcessor: AccentTriage,
		ViewInboxProcessor:       AccentTriage,
		ViewCrossProjectOutliner: AccentTriage,
		ViewCrossProjectKanban:   AccentTriage,
		ViewProjectFocusView:     AccentPlan,
		ViewFocusBoard:           AccentWork,
		ViewDailyReview:          AccentPlan,
		ViewPriorityView:         AccentPlan,
		ViewOutliner:             AccentExecute,
		ViewTemporaryOutliner:    AccentExecute,
		ViewKanban:               AccentExecute,
		ViewWeeklyReview:         AccentReview,
		ViewMetricsDashboard:     AccentReview,
		ViewRecentActivity:       AccentReview,
		ViewRoadmap:              AccentPlan,
		ViewGlobalBlockers:       AccentSupport,
		ViewGlobalNotes:          AccentSupport,
		ViewNotes:                AccentSupport,
		ViewWorkflowRules:        AccentSupport,
		ViewNotificationCenter:   AccentSupport,
		ViewGlobalSearch:         AccentSearch,
		ViewCommandPalette:       AccentSearch,
		ViewPipelineHub:          AccentDashboard,
	}
}

// TestAllViewTypesMatchNames guards allViewTypes() against drift from the
// canonical viewTypeNames map: they must describe exactly the same set.
func TestAllViewTypesMatchNames(t *testing.T) {
	all := allViewTypes()
	if len(all) != len(viewTypeNames) {
		t.Fatalf("allViewTypes has %d entries, viewTypeNames has %d", len(all), len(viewTypeNames))
	}
	seen := make(map[ViewType]bool, len(all))
	for _, vt := range all {
		if seen[vt] {
			t.Errorf("allViewTypes lists %s more than once", vt)
		}
		seen[vt] = true
		if _, ok := viewTypeNames[vt]; !ok {
			t.Errorf("allViewTypes lists %s which is missing from viewTypeNames", vt)
		}
	}
	for vt := range viewTypeNames {
		if !seen[vt] {
			t.Errorf("viewTypeNames has %s which allViewTypes omits", vt)
		}
	}
}

// TestRouteTableCoversEveryViewType ensures the route table has a descriptor for
// every ViewType, so routeFor never falls back to a synthesized default in
// production.
func TestRouteTableCoversEveryViewType(t *testing.T) {
	for _, vt := range allViewTypes() {
		d, ok := routeTable[vt]
		if !ok {
			t.Errorf("route table missing descriptor for %s", vt)
			continue
		}
		if d.ID != vt {
			t.Errorf("descriptor for %s has ID %s", vt, d.ID)
		}
		if d.Name != vt.String() {
			t.Errorf("descriptor for %s has name %q, want %q", vt, d.Name, vt.String())
		}
	}
	if len(routeTable) != len(allViewTypes()) {
		t.Errorf("route table has %d entries, want %d", len(routeTable), len(allViewTypes()))
	}
}

// TestRouteTraitParity proves the declarative table reproduces every legacy
// trait predicate for every ViewType.
func TestRouteTraitParity(t *testing.T) {
	for _, vt := range allViewTypes() {
		if got, want := isTrackedView(vt), legacyIsTrackedView(vt); got != want {
			t.Errorf("isTrackedView(%s) = %v, want %v", vt, got, want)
		}
		if got, want := isGlobalNavView(vt), legacyIsGlobalNavView(vt); got != want {
			t.Errorf("isGlobalNavView(%s) = %v, want %v", vt, got, want)
		}
		if got, want := isTaskScopedView(vt), legacyIsTaskScopedView(vt); got != want {
			t.Errorf("isTaskScopedView(%s) = %v, want %v", vt, got, want)
		}
		if got, want := isGlobalOverlayView(vt), legacyIsGlobalOverlayView(vt); got != want {
			t.Errorf("isGlobalOverlayView(%s) = %v, want %v", vt, got, want)
		}
	}
}

// TestRouteScopeConsistency checks the informational Scope field agrees with the
// operative predicates: ScopeTask iff task-scoped, and ScopeApp for global-nav
// and the dashboard.
func TestRouteScopeConsistency(t *testing.T) {
	for _, vt := range allViewTypes() {
		d := routeFor(vt)
		if legacyIsTaskScopedView(vt) {
			if d.Scope != ScopeTask {
				t.Errorf("%s is task-scoped but Scope=%d", vt, d.Scope)
			}
			continue
		}
		if legacyIsGlobalNavView(vt) || vt == ViewDashboard {
			if d.Scope != ScopeApp {
				t.Errorf("%s is app-level but Scope=%d", vt, d.Scope)
			}
			continue
		}
		if d.Scope != ScopeProject {
			t.Errorf("%s expected ScopeProject but Scope=%d", vt, d.Scope)
		}
	}
}

// TestBuildViewAccentMapParity proves the route-derived accent map matches the
// legacy hand-written accent map exactly after a theme is applied.
func TestBuildViewAccentMapParity(t *testing.T) {
	buildViewAccentMap()
	want := legacyAccentMap()

	if len(viewAccentMap) != len(want) {
		t.Errorf("accent map has %d entries, want %d", len(viewAccentMap), len(want))
	}
	for _, vt := range allViewTypes() {
		got := ViewAccentColor(vt)
		exp := want[vt]
		if got != exp {
			t.Errorf("ViewAccentColor(%s) = %v, want %v", vt, got, exp)
		}
	}
}
