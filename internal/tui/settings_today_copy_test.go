package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestSettingsTodayStartViewCopyPreservesConfigValue(t *testing.T) {
	if got := startViewValues[defaultStartViewCursor]; got != config.DefaultStartView {
		t.Fatalf("default start view value = %q, want %q", got, config.DefaultStartView)
	}
	if got := startViewOptions[defaultStartViewCursor]; got != "Today" {
		t.Fatalf("default start view label = %q, want Today", got)
	}
	for _, label := range startViewOptions {
		if label == "Focus Board" {
			t.Fatal("start view selector should label focus-board as Today")
		}
	}
}

func TestSettingsTodayCopyAvoidsStaleFocusBoardLabels(t *testing.T) {
	model := NewSettingsModel(newSettingsDeps(newTestConfig(), nil))
	model.focusIndex = subTabFieldStart(tabBehavior, 6)
	startup := model.renderBehaviorStartupSection()
	if !strings.Contains(startup, "default: Today cockpit") {
		t.Fatalf("startup settings copy should describe Today default, got:\n%s", startup)
	}
	if strings.Contains(startup, "Focus Board") {
		t.Fatalf("startup settings copy should not expose Focus Board, got:\n%s", startup)
	}

	model.focusIndex = subTabFieldStart(tabBehavior, 8)
	today := model.renderBehaviorFocusBoardSection()
	if !strings.Contains(today, "when Today opens") {
		t.Fatalf("Today settings copy missing Today label, got:\n%s", today)
	}
	if strings.Contains(today, "Focus Board") {
		t.Fatalf("Today settings copy should not expose Focus Board, got:\n%s", today)
	}
}

func TestSettingsSearchLabelsTodaySection(t *testing.T) {
	meta, ok := settingsFieldByIndex(subTabFieldStart(tabBehavior, 8))
	if !ok {
		t.Fatal("missing Today default-mode settings field")
	}
	if meta.TabLabel != "General > Today" {
		t.Fatalf("settings field tab label = %q, want General > Today", meta.TabLabel)
	}
	if meta.Default != "List" {
		t.Fatalf("Today default mode setting default = %q, want List", meta.Default)
	}

	results := searchSettingsFields("today mode")
	if len(results) == 0 || results[0].TabLabel != "General > Today" {
		t.Fatalf("search for today mode should find the Today settings field, got %#v", results)
	}
}
