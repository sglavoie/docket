package outliner

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/textgrammar"
)

// TestQuickEditFooterTeachesEveryPositiveEditField holds the footer table to
// the grammar: a positive field the edit surface gains without an entry would
// otherwise vanish from the one place the inline editor teaches it, and an
// entry spells one positive token and one label, never a list of forms.
func TestQuickEditFooterTeachesEveryPositiveEditField(t *testing.T) {
	positive := 0
	for _, definition := range textgrammar.GuideDefinitions(textgrammar.SurfaceEdit) {
		if strings.HasPrefix(definition.Trigger, "-") {
			if _, ok := quickEditFooterEntries[definition.Field]; ok {
				t.Errorf("footer names the negation %s", definition.Field)
			}
			continue
		}
		positive++
		entry, ok := quickEditFooterEntries[definition.Field]
		if !ok {
			t.Errorf("edit field %s has no footer entry", definition.Field)
			continue
		}
		token, label, found := strings.Cut(entry, " ")
		if !found || label == "" || strings.HasPrefix(token, "-") || strings.Contains(token, "/") || strings.Contains(label, " ") {
			t.Errorf("footer entry %q is not one positive token and one label", entry)
		}
	}
	if got := len(quickEditGrammarGuidance()); got != positive {
		t.Fatalf("footer has %d entries, want one per positive field (%d)", got, positive)
	}
}
