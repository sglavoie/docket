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
		{"backlog", ViewFocusView},
		{"priority", ViewPriorityView},
		{"due-dates", ViewDueDateOverview},
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
