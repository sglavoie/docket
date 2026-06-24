package config

import "testing"

func TestGetStartView_DefaultFocusBoardWhenUnset(t *testing.T) {
	cfg := NewConfig()

	if got := cfg.GetStartView(); got != DefaultStartView {
		t.Errorf("expected DefaultStartView (%q) when StartView is unset, got %q", DefaultStartView, got)
	}
	if DefaultStartView != "focus-board" {
		t.Fatalf("DefaultStartView changed to %q; update the UX redesign session 7 fixtures", DefaultStartView)
	}
}

func TestGetStartView_HonoursExplicitValues(t *testing.T) {
	cases := []string{
		"dashboard",
		"focus-board",
		"cross-project-outliner",
		"all-tasks",
		"daily-review",
		"backlog",
		"priority",
		"due-dates",
	}
	for _, v := range cases {
		cfg := NewConfig()
		cfg.StartView = v
		if got := cfg.GetStartView(); got != v {
			t.Errorf("explicit StartView=%q: expected %q, got %q", v, v, got)
		}
	}
}

func TestGetStartView_ClampsInvalidValues(t *testing.T) {
	cases := []string{
		"",
		"unknown",
		"DASHBOARD",
		"focus_board",
		"task-tree",
	}
	for _, v := range cases {
		cfg := NewConfig()
		cfg.StartView = v
		if got := cfg.GetStartView(); got != DefaultStartView {
			t.Errorf("invalid StartView=%q: expected clamp to %q, got %q", v, DefaultStartView, got)
		}
	}
}

// TestGetStartView_ExplicitDashboardStillHonoured guards the contract that
// users who deliberately pinned the legacy Dashboard remain on it after the
// Today-cockpit default lands; only unset/invalid values move to Focus Board.
func TestGetStartView_ExplicitDashboardStillHonoured(t *testing.T) {
	cfg := NewConfig()
	cfg.StartView = "dashboard"
	if got := cfg.GetStartView(); got != "dashboard" {
		t.Errorf("explicit dashboard should round-trip: got %q", got)
	}
}
