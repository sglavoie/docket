package tui

import (
	"strings"
	"testing"
)

// TestProjectTreeActiveFilterParts_ReadyAppearsOnce guards against a regression
// where toggling the Actionable filter resulted in "ready" being appended both
// by commonFilterParts() and by the view-specific activeFilterParts(), inflating
// ActiveFilterCount() and producing "Filters: ready ready" in the badge.
func TestProjectTreeActiveFilterParts_ReadyAppearsOnce(t *testing.T) {
	app, project := newTestAppModelWithProject()
	pt := NewProjectTreeModel(project, app.config, nil)
	pt.filter.Actionable = true

	parts := pt.activeFilterParts()
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) == "ready" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'ready' filter part, got %d (parts: %v)", count, parts)
	}
	if pt.ActiveFilterCount() != len(parts) {
		t.Errorf("ActiveFilterCount = %d, want %d", pt.ActiveFilterCount(), len(parts))
	}
}

// TestOutlinerActiveFilterParts_ReadyAppearsOnce is the outliner counterpart of
// TestProjectTreeActiveFilterParts_ReadyAppearsOnce.
func TestOutlinerActiveFilterParts_ReadyAppearsOnce(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.config, app.configManager)
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

// TestProjectTreeActiveFilterParts_NoReadyWhenInactive ensures "ready" is absent
// when Actionable is not toggled on.
func TestProjectTreeActiveFilterParts_NoReadyWhenInactive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	pt := NewProjectTreeModel(project, app.config, nil)
	pt.filter.Actionable = false

	for _, p := range pt.activeFilterParts() {
		if strings.TrimSpace(p) == "ready" {
			t.Errorf("expected no 'ready' part when Actionable is false, got %v", pt.activeFilterParts())
			break
		}
	}
}
