package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCommonFilterParts_TagsDeterministicOrder guards against a regression where
// commonFilterParts() iterated a map and produced a nondeterministic display
// order in the filter badge for the Tags filter.
func TestCommonFilterParts_TagsDeterministicOrder(t *testing.T) {
	f := newFilterState()
	f.Tags["zebra"] = true
	f.Tags["alpha"] = true
	f.Tags["mango"] = true
	f.Tags["banana"] = true

	// commonFilterParts may include other parts; we only assert the tag part.
	// Locate the parts entry that contains tag markers and ensure it is
	// identical across many calls.
	first := tagPartFromCommonFilterParts(commonFilterParts(&f))
	if first == "" {
		t.Fatalf("expected a tag part in commonFilterParts output, got none")
	}
	// Tags should appear in lexicographic order: #alpha #banana #mango #zebra
	wantTagPart := "#alpha #banana #mango #zebra"
	if first != wantTagPart {
		t.Errorf("tag part not in deterministic sorted order: got %q, want %q", first, wantTagPart)
	}

	// Repeat many times to detect map-iteration nondeterminism.
	for i := range 50 {
		got := tagPartFromCommonFilterParts(commonFilterParts(&f))
		if got != first {
			t.Fatalf("iteration %d: tag part changed across calls: got %q, want %q", i, got, first)
		}
	}
}

func tagPartFromCommonFilterParts(parts []string) string {
	for _, p := range parts {
		if strings.HasPrefix(p, "#") {
			return p
		}
	}
	return ""
}

// TestOutlinerActiveFilterParts_StatesCanonicalOrder guards the canonical
// state-set display order used by the active filter chip.
func TestOutlinerActiveFilterParts_StatesCanonicalOrder(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.setStateFilter(testWorkflowStateSet(
		models.StateCanceled,
		models.StateBacklog,
		models.StateInbox,
		models.StateWaiting,
	), true)

	first := statePartFromParts(ol.activeFilterParts())
	if first == "" {
		t.Fatalf("expected a state part in activeFilterParts output, got none")
	}
	wantStatePart := "state:inbox,backlog,waiting,canceled"
	if first != wantStatePart {
		t.Errorf("state part not in canonical order: got %q, want %q", first, wantStatePart)
	}

	for i := range 50 {
		got := statePartFromParts(ol.activeFilterParts())
		if got != first {
			t.Fatalf("iteration %d: state part changed across calls: got %q, want %q", i, got, first)
		}
	}
}

func statePartFromParts(parts []string) string {
	for _, p := range parts {
		if strings.HasPrefix(p, "state:") {
			return p
		}
	}
	return ""
}
