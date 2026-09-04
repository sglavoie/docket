package help

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// guidanceText and assertGuidance are this package's own copies of the two
// helpers internal/tui/overlay_guidance_test.go declares. A test helper cannot
// cross a package boundary without becoming production-unused exported API, so
// the four lines are duplicated rather than promoted.
func guidanceText(spec chrome.GuidanceSpec) string {
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, chrome.GuidanceHintText(hint))
	}
	return strings.Join(entries, " | ") + " | " + spec.Supplemental
}

func assertGuidance(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance unavailable", state)
	}
	text := guidanceText(spec)
	for _, entry := range want {
		if !strings.Contains(text, entry) {
			t.Errorf("%s guidance missing %q: %s", state, entry, text)
		}
	}
}

func TestHelpGuidanceTracksBrowseSearchTabAndEmptyStates(t *testing.T) {
	model := NewHelpViewModel([]operation.HelpItem{{Key: "j", Description: "Move", Category: "Navigate"}}, "Outliner")
	model.SetSize(90, 24)

	spec, ok := model.ContextualGuidance()
	assertGuidance(t, "browse", spec, ok, "? close", "/ filter", "\\ find tab", "j/k scroll")
	if spec.Rows != 1 {
		t.Fatalf("browse rows = %d, want 1", spec.Rows)
	}

	model.searchMode = true
	spec, ok = model.ContextualGuidance()
	assertGuidance(t, "filter edit", spec, ok, "Enter apply filter", "Esc cancel edit", "\\ find tab")
	if spec.Rows != 2 {
		t.Fatalf("filter-edit rows = %d, want 2", spec.Rows)
	}

	model.searchMode = false
	model.tabSearchMode = true
	spec, ok = model.ContextualGuidance()
	assertGuidance(t, "tab find", spec, ok, "Enter apply tab", "Esc cancel find", "/ edit filter")

	model.tabSearchMode = false
	model.filteredItems = nil
	spec, ok = model.ContextualGuidance()
	assertGuidance(t, "empty", spec, ok, "no matching help items", "? close")
}
