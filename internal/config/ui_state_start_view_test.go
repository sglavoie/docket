package config

import "testing"

func TestGetStartViewDefaultsToPlanner(t *testing.T) {
	machine := NewMachineState()
	if got := machine.GetStartView(); got != StartViewPlanner {
		t.Fatalf("default start view = %q, want planner", got)
	}
}

func TestGetStartViewHonoursCanonicalValues(t *testing.T) {
	values := []string{
		StartViewHome,
		StartViewPlanner,
		StartViewTasks,
		StartViewProject,
		StartViewReview,
		StartViewManage,
	}
	for _, value := range values {
		machine := NewMachineState()
		machine.SetStartView(value)
		if got := machine.GetStartView(); got != value {
			t.Fatalf("SetStartView(%q) = %q", value, got)
		}
	}
}

func TestGetStartViewClampsInvalidAndLegacyValuesUntilMigration(t *testing.T) {
	for _, value := range []string{"", "unknown", "dashboard", "focus-board", "all-tasks"} {
		machine := NewMachineState()
		machine.StartView = value
		if got := machine.GetStartView(); got != DefaultStartView {
			t.Fatalf("GetStartView(%q) = %q, want %q", value, got, DefaultStartView)
		}
	}
}
