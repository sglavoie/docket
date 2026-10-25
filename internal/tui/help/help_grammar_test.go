package help

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/textgrammar"
)

// TestHelpGrammarSectionComesFromCanonicalRegistry holds the `?` grammar section
// to the same teach order the composer guide reads, over the union of the three
// surfaces a sigil can be typed on. It is the second GuideDefinitions consumer,
// so a new sigil that reaches the composer must reach `?` in the same pass.
//
// It sits here rather than beside the composer half of the same contract in
// internal/tui/textgrammar_consumer_contract_test.go because every declaration
// it names is this package's: the section constant, the projection, and the key
// column width the row has to fit.
func TestHelpGrammarSectionComesFromCanonicalRegistry(t *testing.T) {
	surfaces := textgrammar.SurfaceCapture | textgrammar.SurfaceEdit | textgrammar.SurfaceSearch
	definitions := textgrammar.GuideDefinitions(surfaces)
	items := GrammarHelpItems()
	if len(items) != len(definitions) {
		t.Fatalf("grammar help rows = %d, want %d registry definitions", len(items), len(definitions))
	}
	m := NewHelpViewModel(items, "Outliner")
	m.SetSize(60, 30)
	rendered := ansi.Strip(m.renderContent())
	for i, definition := range definitions {
		item := items[i]
		if got, want := item.Key, strings.Join(definition.Forms, " "); got != want {
			t.Errorf("%s key = %q, want %q", definition.Field, got, want)
		}
		if !strings.HasPrefix(item.Description, definition.Description) {
			t.Errorf("%s description = %q, want it to lead with %q", definition.Field, item.Description, definition.Description)
		}
		for _, clear := range definition.Clears {
			if !strings.Contains(item.Description, clear.Description) {
				t.Errorf("%s description = %q, want it to name the clear form %q", definition.Field, item.Description, clear.Description)
			}
		}
		if item.Category != helpGrammarCategory {
			t.Errorf("%s category = %q, want %q", definition.Field, item.Category, helpGrammarCategory)
		}
		if !strings.Contains(rendered, item.Key) {
			t.Errorf("%s key %q was split or lost", definition.Field, item.Key)
		}
		for _, alias := range item.Aliases {
			if strings.TrimSpace(alias) == "" {
				t.Errorf("%s carries a blank alias", definition.Field)
			}
		}
	}

	// The deferral sigil is the whole reason the section exists: it has no key of
	// its own, so `?` is the only place outside the composer that can teach it.
	// The row is found by prefix rather than by substring: -@@defer is a row of its
	// own now, and its key contains the positive form's spelling.
	deferRow := -1
	for i, item := range items {
		if strings.HasPrefix(item.Key, "@@defer") {
			deferRow = i
		}
	}
	if deferRow < 0 {
		t.Fatalf("grammar help omits @@defer: %#v", items)
	}
	if !slices.Contains(items[deferRow].Aliases, "@@any") || !slices.Contains(items[deferRow].Aliases, "@@none") {
		t.Errorf("deferral aliases = %v, want the search-only presets", items[deferRow].Aliases)
	}

	// The @+Nh due window is search-only, so `?` is its only in-app advertising
	// surface beside the README. It rides the due row as a searchable alias the
	// way the deferral presets ride the defer row.
	dueRow := -1
	for i, item := range items {
		if strings.HasPrefix(item.Key, "@date") {
			dueRow = i
		}
	}
	if dueRow < 0 {
		t.Fatalf("grammar help omits @date: %#v", items)
	}
	if !slices.Contains(items[dueRow].Aliases, "@+4h") {
		t.Errorf("due aliases = %v, want the search-only @+4h hour window", items[dueRow].Aliases)
	}
	if !strings.Contains(items[dueRow].Description, "@+4h") {
		t.Errorf("due description = %q, want it to visibly show the @+4h search window", items[dueRow].Description)
	}
	if !strings.Contains(items[deferRow].Description, "@@any") || !strings.Contains(items[deferRow].Description, "@@none") {
		t.Errorf("deferral description = %q, want it to visibly show the @@any/@@none search forms", items[deferRow].Description)
	}
}
