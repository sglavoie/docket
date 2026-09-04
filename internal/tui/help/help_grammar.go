package help

import (
	"strings"

	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// helpGrammarCategory is the `?` section the shared sigil grammar renders under.
// It is a section name rather than an OperationCategory because the grammar is
// not an operation: no key dispatches it and the registry never sees it.
const helpGrammarCategory = "Capture & Edit Grammar"

// GrammarHelpItems projects the shared inline grammar into `?` help rows, giving
// textgrammar.GuideDefinitions a second consumer beside the capture composer's
// syntax guide. Without it the sigils are documented only behind `?` inside `a`,
// so a user who never opens the composer cannot discover @@defer at all.
//
// The surface mask is the union of the three surfaces a sigil can be typed on.
// formsForSurface masks with &, so a union returns every live spelling of a
// field — "@@defer" and the edit-only "@@0" on one row — and Definitions only
// swaps in the search wording for the bare SurfaceSearch mask, which keeps these
// rows describing what a sigil does rather than what it matches. Legacy entries
// are already excluded by Definitions.
func GrammarHelpItems() []operation.HelpItem {
	surfaces := textgrammar.SurfaceCapture | textgrammar.SurfaceEdit | textgrammar.SurfaceSearch
	definitions := textgrammar.GuideDefinitions(surfaces)
	items := make([]operation.HelpItem, 0, len(definitions))
	for _, def := range definitions {
		description := def.Description
		for _, clear := range def.Clears {
			description += " · " + clear.Description
		}
		if forms := grammarHelpSearchForms(def); forms != "" {
			description += " · search: " + forms
		}
		items = append(items, operation.HelpItem{
			Key:         strings.Join(def.Forms, " "),
			Description: description,
			Category:    helpGrammarCategory,
			Aliases:     grammarHelpAliases(def),
		})
	}
	return items
}

// grammarHelpSearchForms renders the search-only spellings that appear nowhere
// else on screen so `?` shows them rather than hiding them behind an exact-string
// alias search. Only the due and deferral sigils carry such forms today: the due
// relational buckets and the `@+Nh` window, and the deferral presence split. The
// forms read from the canonical search lists, so this row can never drift from
// what ParseSearch honors. Every other field's search vocabulary is identical to
// its capture vocabulary and so is already visible in the key column.
func grammarHelpSearchForms(def textgrammar.Definition) string {
	presets := textgrammar.SearchPresetValues(def.Field)
	examples := textgrammar.SearchExampleValues(def.Field)
	forms := make([]string, 0, len(presets)+len(examples))
	for _, preset := range presets {
		forms = append(forms, def.Trigger+preset)
	}
	for _, example := range examples {
		forms = append(forms, def.Trigger+example)
	}
	return strings.Join(forms, " ")
}

// grammarHelpAliases is every literal a user might type into `?` search looking
// for this sigil: the taught examples, the preset values, the relational values
// only search understands, and the search-only parametric examples such as the
// due `@+4h` hour window. Appending the search presets is what puts
// "how do I list what is hidden" in `?`, since @@any and @@none appear nowhere
// else in the app, and the search examples do the same for the `@+Nh` window,
// which is search-only and so cannot ride the shared capture Examples. They are
// aliases rather than description text because the description column is one
// line wide and the value lists are long.
//
// A bare value is trigger-prefixed so every alias is a string the user could
// have typed, which is also what lets a query for "@@none" reach the deferral
// row rather than the due row that shares the bare word.
func grammarHelpAliases(def textgrammar.Definition) []string {
	presets := textgrammar.SearchPresetValues(def.Field)
	examples := textgrammar.SearchExampleValues(def.Field)
	aliases := make([]string, 0, len(def.Examples)+len(def.Values)+len(presets)+len(examples))
	seen := make(map[string]bool, cap(aliases))
	add := func(alias string) {
		key := strings.ToLower(alias)
		if alias == "" || seen[key] {
			return
		}
		seen[key] = true
		aliases = append(aliases, alias)
	}
	for _, example := range def.Examples {
		add(example)
	}
	for _, value := range def.Values {
		add(def.Trigger + value)
	}
	for _, preset := range presets {
		add(def.Trigger + preset)
	}
	for _, example := range examples {
		add(def.Trigger + example)
	}
	return aliases
}
