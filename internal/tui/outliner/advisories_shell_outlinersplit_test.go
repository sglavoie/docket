package outliner

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
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
		for _, entry := range viewMenuEntries(chrome.ModeAll, true, visible) {
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
