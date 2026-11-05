package outliner

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// The shell half of the advisory coverage: the `--no-advisories` option, the
// keyless View-menu toggle, and the checkbox that reads it back. What an
// advisory is and how the two helpers gate one stays in advisories_test.go.

// TestAdvisoriesMenuRowCarriesLiveVisibility keeps the checkbox honest: it reads
// shown, matching how the bar's own row reads, so the dropdown describes state
// rather than an action.
func TestAdvisoriesMenuRowCarriesLiveVisibility(t *testing.T) {
	for _, visible := range []bool{false, true} {
		var found bool
		for _, entry := range viewMenuEntries(chrome.ModeAll, ViewMenuChecks{MenuBarVisible: true, AdvisoriesVisible: visible, MouseEnabled: true}) {
			if entry.OperationID != operation.OperationCoreToggleAdvisories {
				continue
			}
			found = true
			if entry.Checked == nil {
				t.Fatal("the advisories row carries no checkbox state")
			}
			if *entry.Checked != visible {
				t.Errorf("visible=%v resolved checked=%v", visible, *entry.Checked)
			}
		}
		if !found {
			t.Fatalf("visible=%v: the View menu does not offer the advisories toggle", visible)
		}
	}
}

// TestMouseMenuRowCarriesLiveState holds the Mouse row to the same reading:
// ticked is the mouse on, and the row is keyless chrome like Advisories.
func TestMouseMenuRowCarriesLiveState(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var found bool
		for _, entry := range viewMenuEntries(chrome.ModeAll, ViewMenuChecks{MenuBarVisible: true, AdvisoriesVisible: true, MouseEnabled: enabled}) {
			if entry.OperationID != operation.OperationCoreToggleMouse {
				continue
			}
			found = true
			if entry.Label != "Mouse" || entry.Checked == nil {
				t.Fatalf("the mouse row is %+v", entry)
			}
			if *entry.Checked != enabled {
				t.Errorf("enabled=%v resolved checked=%v", enabled, *entry.Checked)
			}
		}
		if !found {
			t.Fatalf("enabled=%v: the View menu does not offer the mouse toggle", enabled)
		}
	}
}

// TestMouseEnabledReadsTheInjectedSetting keeps a model with no shell honest:
// no accessor reads as on, the default a machine that never toggled it has.
func TestMouseEnabledReadsTheInjectedSetting(t *testing.T) {
	m := &OutlinerModel{}
	if !m.MouseEnabled() {
		t.Fatal("a model with no accessor reads the mouse as off")
	}
	m.SetMouseDisabled(func() bool { return true })
	if m.MouseEnabled() {
		t.Fatal("the injected setting was not read")
	}
}

// TestPresentationPreferenceRowsCarryLiveState holds Motion and Nerd Font
// Glyphs to the same reading: each row ticks its own field and no other.
func TestPresentationPreferenceRowsCarryLiveState(t *testing.T) {
	rows := map[operation.OperationID]string{
		operation.OperationCoreToggleMotion:     "Motion",
		operation.OperationCoreToggleNerdGlyphs: "Nerd Font Glyphs",
	}
	for _, checks := range []ViewMenuChecks{{MotionEnabled: true}, {NerdGlyphs: true}, {}} {
		want := map[operation.OperationID]bool{
			operation.OperationCoreToggleMotion:     checks.MotionEnabled,
			operation.OperationCoreToggleNerdGlyphs: checks.NerdGlyphs,
		}
		found := 0
		for _, entry := range viewMenuEntries(chrome.ModeAll, checks) {
			label, ok := rows[entry.OperationID]
			if !ok {
				continue
			}
			found++
			if entry.Label != label || entry.Checked == nil {
				t.Fatalf("the %s row is %+v", label, entry)
			}
			if *entry.Checked != want[entry.OperationID] {
				t.Errorf("%+v: %s checked=%v", checks, label, *entry.Checked)
			}
		}
		if found != len(rows) {
			t.Fatalf("%+v: the View menu offers %d of the %d preference rows", checks, found, len(rows))
		}
	}
	m := &OutlinerModel{}
	if !m.MotionPreferred() || m.NerdGlyphs() {
		t.Fatal("a model with no accessors reads motion off or the Nerd set")
	}
	m.SetPresentationPreferences(func() bool { return false }, func() bool { return true })
	if m.MotionPreferred() || !m.NerdGlyphs() {
		t.Fatal("the injected preferences were not read")
	}
}
