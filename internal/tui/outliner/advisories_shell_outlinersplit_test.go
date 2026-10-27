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
		for _, entry := range viewMenuEntries(chrome.ModeAll, true, visible, true) {
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
		for _, entry := range viewMenuEntries(chrome.ModeAll, true, true, enabled) {
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
