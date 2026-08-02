package tui

import (
	"strings"
	"testing"
)

// TestOutlinerActiveFilterParts_ActionableAppearsOnce guards against a regression
// where toggling the Actionable filter resulted in "actionable" being appended both
// by commonFilterParts() and by the view-specific activeFilterParts(), inflating
// ActiveFilterCount() and producing "Filters: actionable actionable" in the badge.
func TestOutlinerActiveFilterParts_ActionableAppearsOnce(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.Actionable = true

	parts := ol.activeFilterParts()
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) == "actionable" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'actionable' filter part, got %d (parts: %v)", count, parts)
	}
	if ol.ActiveFilterCount() != len(parts) {
		t.Errorf("ActiveFilterCount = %d, want %d", ol.ActiveFilterCount(), len(parts))
	}
}

// TestOutlinerActiveFilterParts_NoActionableWhenInactive ensures "actionable" is absent
// when Actionable is not toggled on.
func TestOutlinerActiveFilterParts_NoActionableWhenInactive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.Actionable = false

	for _, p := range ol.activeFilterParts() {
		if strings.TrimSpace(p) == "actionable" {
			t.Errorf("expected no 'actionable' part when Actionable is false, got %v", ol.activeFilterParts())
			break
		}
	}
}
