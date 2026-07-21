package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestSettingsPlannerStartViewCopyPreservesConfigValue(t *testing.T) {
	if got := startViewValues[defaultStartViewCursor]; got != config.DefaultStartView {
		t.Fatalf("default start view value = %q, want %q", got, config.DefaultStartView)
	}
	if got := startViewOptions[defaultStartViewCursor]; got != "Planner" {
		t.Fatalf("default start view label = %q, want Planner", got)
	}
}

func TestSettingsStartupCopyAvoidsStaleFocusBoardLabels(t *testing.T) {
	model := NewSettingsModel(newSettingsDeps(newTestConfig(), nil))
	model.focusIndex = subTabFieldStart(tabBehavior, 5)
	startup := model.renderBehaviorStartupSection()
	if !strings.Contains(startup, "default: Planner") {
		t.Fatalf("startup settings copy should describe Planner default, got:\n%s", startup)
	}
	if strings.Contains(startup, "Focus Board") {
		t.Fatalf("startup settings copy should not expose Focus Board, got:\n%s", startup)
	}
}

func TestSettingsSearchRetiresTodayPresentationControl(t *testing.T) {
	if results := searchSettingsFields("today mode"); len(results) != 0 {
		t.Fatalf("search for retired Today mode returned %#v", results)
	}
}
