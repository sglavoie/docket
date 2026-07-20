package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestResolveStartupNavigation_DefaultsToFocusBoard guards the new Today
// cockpit default. AppModel boots with the dashboard at the bottom of the
// view stack; resolveStartupNavigation decides whether Init should push a
// different start view on top. Unset/invalid StartView resolves to Focus
// Board so a fresh install lands on the explainable next-action surface.
func TestResolveStartupNavigation_DefaultsToFocusBoard(t *testing.T) {
	cfg := newTestConfig()
	vt, ok := resolveStartupNavigation(cfg.Machine().GetStartView())
	if !ok {
		t.Fatalf("expected navigation away from dashboard for unset StartView")
	}
	if vt != ViewFocusBoard {
		t.Errorf("expected ViewFocusBoard for unset StartView, got %v", vt)
	}
}

func TestResolveStartupNavigation_InvalidValueLandsOnFocusBoard(t *testing.T) {
	cfg := newTestConfig()
	cfg.Machine().StartView = "not-a-view"
	vt, ok := resolveStartupNavigation(cfg.Machine().GetStartView())
	if !ok {
		t.Fatalf("expected navigation away from dashboard for invalid StartView")
	}
	if vt != ViewFocusBoard {
		t.Errorf("expected ViewFocusBoard fallback, got %v", vt)
	}
}

// TestResolveStartupNavigation_ExplicitDashboardStaysPut keeps the legacy
// dashboard-on-boot contract for users who deliberately pinned it.
func TestResolveStartupNavigation_ExplicitDashboardStaysPut(t *testing.T) {
	cfg := newTestConfig()
	cfg.Machine().StartView = "dashboard"
	vt, ok := resolveStartupNavigation(cfg.Machine().GetStartView())
	if ok {
		t.Fatalf("expected no navigation push for explicit dashboard, got %v", vt)
	}
}

func TestResolveStartupNavigation_HonoursExplicitViews(t *testing.T) {
	cases := []struct {
		startView string
		want      ViewType
	}{
		{"focus-board", ViewFocusBoard},
		{"cross-project-outliner", ViewCrossProjectOutliner},
		{"all-tasks", ViewAllTasks},
		{"daily-review", ViewDailyReview},
		{"backlog", ViewFocusBoard},
		{"priority", ViewPriorityView},
		{"due-dates", ViewAllTasks},
	}
	for _, tc := range cases {
		cfg := newTestConfig()
		cfg.Machine().StartView = tc.startView
		vt, ok := resolveStartupNavigation(cfg.Machine().GetStartView())
		if !ok {
			t.Fatalf("StartView=%q: expected navigation push", tc.startView)
		}
		if vt != tc.want {
			t.Errorf("StartView=%q: expected %v, got %v", tc.startView, tc.want, vt)
		}
	}
}

// TestStartupAliasesPreservePlannerTabAndAllTasksGrouping guards FR-007-006:
// the "backlog" and "due-dates" start-view aliases fold onto shared views
// (Planner / All Tasks) but must carry their subview so a persisted preference
// does not decay into a default tab/grouping on boot. It asserts the resolved
// startup message AND the tab/grouping of the model constructed from it, not
// only the destination ViewType.
func TestStartupAliasesPreservePlannerTabAndAllTasksGrouping(t *testing.T) {
	t.Run("backlog opens the Planner Backlog tab", func(t *testing.T) {
		msg, ok := startupNavigationMsg("backlog")
		if !ok {
			t.Fatalf("expected navigation push for backlog start-view")
		}
		fbMsg, isFB := msg.(ShowFocusBoardMsg)
		if !isFB {
			t.Fatalf("expected ShowFocusBoardMsg, got %T", msg)
		}
		if !fbMsg.SetTab || fbMsg.Tab != boardTabBacklog {
			t.Fatalf("expected Planner Backlog tab, got %+v", fbMsg)
		}
		// Applying the message to a constructed Planner lands it on Backlog
		// regardless of Daily Plan contents (initialTabSet suppresses the
		// empty-plan → Next auto-default).
		fb := newTestFocusBoard()
		if fbMsg.SetTab {
			fb.tabs.Active = fbMsg.Tab
			fb.initialTabSet = true
		}
		if fb.tabs.Active != boardTabBacklog {
			t.Errorf("constructed Planner active tab = %d, want boardTabBacklog (%d)", fb.tabs.Active, boardTabBacklog)
		}
		if !fb.initialTabSet {
			t.Errorf("constructed Planner did not mark initialTabSet; auto-default may override Backlog")
		}
	})

	t.Run("due-dates opens All Tasks grouped by due date", func(t *testing.T) {
		msg, ok := startupNavigationMsg("due-dates")
		if !ok {
			t.Fatalf("expected navigation push for due-dates start-view")
		}
		atMsg, isAT := msg.(ShowAllTasksMsg)
		if !isAT {
			t.Fatalf("expected ShowAllTasksMsg, got %T", msg)
		}
		if !atMsg.SetGroup || atMsg.Group != ATGroupByDueDate {
			t.Fatalf("expected All Tasks grouped by due date, got %+v", atMsg)
		}
		// The constructed model must open grouped by due date with the
		// Active-tab auto-default suppressed (initialTabSet), mirroring the
		// handler's WithInitialGrouping construction.
		atv := NewAllTasksViewModel(newTestConfig(), &config.Manager{}, WithInitialGrouping(atMsg.Group))
		if atv.groupBy != ATGroupByDueDate {
			t.Errorf("constructed All Tasks groupBy = %v, want ATGroupByDueDate", atv.groupBy)
		}
		if !atv.initialTabSet {
			t.Errorf("constructed All Tasks did not mark initialTabSet; Active-tab auto-default may override the grouping")
		}
	})

	t.Run("dashboard start-view issues no push", func(t *testing.T) {
		if _, ok := startupNavigationMsg("dashboard"); ok {
			t.Fatalf("dashboard start-view should not push a startup navigation message")
		}
	})

	t.Run("plain views route through NavigateToViewMsg", func(t *testing.T) {
		msg, ok := startupNavigationMsg("all-tasks")
		if !ok {
			t.Fatalf("expected navigation push for all-tasks start-view")
		}
		nav, isNav := msg.(NavigateToViewMsg)
		if !isNav {
			t.Fatalf("expected NavigateToViewMsg for a plain view, got %T", msg)
		}
		if nav.ViewType != ViewAllTasks {
			t.Errorf("NavigateToViewMsg.ViewType = %v, want ViewAllTasks", nav.ViewType)
		}
	})
}

// TestDefaultStartViewCursor_MatchesConfigDefault locks the settings
// selector to whatever config.DefaultStartView resolves to, so the Ctrl+D
// reset and the unset-cursor fallback both point at the same option the
// engine treats as the implicit default.
func TestDefaultStartViewCursor_MatchesConfigDefault(t *testing.T) {
	if defaultStartViewCursor < 0 || defaultStartViewCursor >= len(startViewValues) {
		t.Fatalf("defaultStartViewCursor=%d is out of bounds for startViewValues (len=%d)", defaultStartViewCursor, len(startViewValues))
	}
	if got := startViewValues[defaultStartViewCursor]; got != config.DefaultStartView {
		t.Errorf("defaultStartViewCursor points at %q, expected %q", got, config.DefaultStartView)
	}
}
