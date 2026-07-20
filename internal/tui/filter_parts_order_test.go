package tui

import (
	"strings"
	"testing"
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

// TestOutlinerActiveFilterParts_SectionsDeterministicOrder guards against
// map-iteration nondeterminism bug for the per-view Sections filter in
// OutlinerModel.activeFilterParts().
func TestOutlinerActiveFilterParts_SectionsDeterministicOrder(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.Sections = map[string]bool{
		"Zeta":   true,
		"Alpha":  true,
		"Middle": true,
		"Beta":   true,
	}

	first := sectionPartFromParts(ol.activeFilterParts())
	if first == "" {
		t.Fatalf("expected a section part in activeFilterParts output, got none")
	}
	wantSecPart := "@Alpha @Beta @Middle @Zeta"
	if first != wantSecPart {
		t.Errorf("section part not in deterministic sorted order: got %q, want %q", first, wantSecPart)
	}

	for i := range 50 {
		got := sectionPartFromParts(ol.activeFilterParts())
		if got != first {
			t.Fatalf("iteration %d: section part changed across calls: got %q, want %q", i, got, first)
		}
	}
}

func sectionPartFromParts(parts []string) string {
	for _, p := range parts {
		if strings.HasPrefix(p, "@") {
			return p
		}
	}
	return ""
}
