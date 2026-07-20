package tui

import (
	"strings"
	"testing"
)

// TestOutlinerActiveFilterParts_ReadyAppearsOnce guards against a regression
// where toggling the Actionable filter resulted in "ready" being appended both
// by commonFilterParts() and by the view-specific activeFilterParts(), inflating
// ActiveFilterCount() and producing "Filters: ready ready" in the badge.
func TestOutlinerActiveFilterParts_ReadyAppearsOnce(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.Actionable = true

	parts := ol.activeFilterParts()
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) == "ready" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'ready' filter part, got %d (parts: %v)", count, parts)
	}
	if ol.ActiveFilterCount() != len(parts) {
		t.Errorf("ActiveFilterCount = %d, want %d", ol.ActiveFilterCount(), len(parts))
	}
}

// TestOutlinerActiveFilterParts_NoReadyWhenInactive ensures "ready" is absent
// when Actionable is not toggled on.
func TestOutlinerActiveFilterParts_NoReadyWhenInactive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.Actionable = false

	for _, p := range ol.activeFilterParts() {
		if strings.TrimSpace(p) == "ready" {
			t.Errorf("expected no 'ready' part when Actionable is false, got %v", ol.activeFilterParts())
			break
		}
	}
}
