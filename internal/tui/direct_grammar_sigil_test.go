package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// sigilKeyRoute is how one text-grammar trigger reaches the keyboard.
type sigilKeyRoute int

const (
	sigilRootKey sigilKeyRoute = iota
	sigilChordPrefix
	sigilPickerOnly
	// sigilSearchOnly is a trigger that exists nowhere but the search box. It has
	// no keyboard route at all, and unlike a picker-only sigil the keymap is free
	// to bind the same character to an unrelated act: a query is typed into an
	// input, where root keys do not resolve. ? is exactly that — it asks for help
	// on a row and names a computed property in a query.
	sigilSearchOnly
	// sigilFacetOnly is a trigger that writes a field on every mutation surface
	// but takes no root key. Review is the one: $review is a refused destination
	// and no key sets the flag, so the only keyboard route the sigil has is the
	// filter facet spelled with the same character, which narrows rather than
	// writes.
	sigilFacetOnly
)

// grammarSigilRoutes pairs every advertised text-grammar trigger with the route
// the Outliner gives it. The eleven keyed sigils are the correspondence itself:
// the character that writes a field in a capture line is the key that edits it
// on a row. The four picker-only triggers are named rather than omitted so a
// new sigil forces a deliberate decision instead of silently passing, and a
// renamed one breaks the keymap loudly.
var grammarSigilRoutes = map[string]sigilKeyRoute{
	"!": sigilChordPrefix,
	"#": sigilRootKey,
	"@": sigilRootKey,
	"$": sigilChordPrefix,
	"*": sigilRootKey,
	"%": sigilRootKey,
	">": sigilRootKey,
	"<": sigilRootKey,
	// The attachment and comment sigils open their panels rather than a picker:
	// the panel already owns add, reorder, unlink and delete, so the key that
	// writes the field at capture time is the key that reaches it on a row.
	"&": sigilRootKey,
	";": sigilRootKey,
	// Two runes are not one key event, and the # picker already unchecks a tag.
	"-#": sigilPickerOnly,
	// The reference sigil opens a picker over the tasks the cursor task may name,
	// with its stored set preselected, so the character that writes a reference at
	// capture time is the key that edits the set on a row.
	"~": sigilRootKey,
	// Two runes are not one key event, and unchecking a preselected row in that
	// picker already carries the removal `-~` spells.
	"-~": sigilPickerOnly,
	// The trie is prefix-free, so ## and @@ cannot be chords under keys # and @
	// that already resolve. Both live as modes inside the picker their first
	// character opens: ## is the context radio, @@ the defer section.
	"##": sigilPickerOnly,
	"@@": sigilPickerOnly,
	// The remaining negations subtract from a result set rather than from a task,
	// so they exist only where a result set does. Their positive twins keep the
	// keyboard route; there is no row act for "not this priority".
	"-!":  sigilSearchOnly,
	"-##": sigilSearchOnly,
	"-@":  sigilSearchOnly,
	"-@@": sigilSearchOnly,
	"-$":  sigilSearchOnly,
	"-*":  sigilSearchOnly,
	"-=":  sigilSearchOnly,
	// The review sigil has no setter key at all; f= is its whole keyboard route.
	"=":  sigilFacetOnly,
	"-%": sigilSearchOnly,
	// A derived property is computed from the task rather than stored on it, so
	// there is nothing for a key to write and both spellings stay in the query.
	"?":  sigilSearchOnly,
	"-?": sigilSearchOnly,
}

func TestSigilKeysMatchTextGrammarSigils(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	rootKeys := make(map[string]operation.OperationID)
	prefixes := make(map[string]operation.OperationID)
	for _, binding := range strategy.Bindings {
		for _, key := range binding.Keys {
			rootKeys[key] = binding.OperationID
		}
		if len(binding.Sequence) > 0 {
			prefixes[binding.Sequence[0]] = binding.OperationID
		}
	}

	declared := make(map[string]bool, len(grammarSigilRoutes))
	for _, surface := range []textgrammar.Surface{
		textgrammar.SurfaceCapture,
		textgrammar.SurfaceEdit,
		textgrammar.SurfaceOutline,
		textgrammar.SurfaceSearch,
	} {
		for _, definition := range textgrammar.Definitions(surface) {
			trigger := definition.Trigger
			if declared[trigger] {
				continue
			}
			declared[trigger] = true
			route, named := grammarSigilRoutes[trigger]
			if !named {
				t.Errorf(
					"text grammar trigger %q (%s) has no declared keyboard route; bind it or name it picker-only",
					trigger,
					definition.Field,
				)
				continue
			}
			_, isRoot := rootKeys[trigger]
			_, isPrefix := prefixes[trigger]
			switch route {
			case sigilRootKey:
				if !isRoot {
					t.Errorf("sigil %q (%s) is not bound as a root key", trigger, definition.Field)
				}
				if isPrefix {
					t.Errorf("sigil %q (%s) is a root key and a chord prefix at once", trigger, definition.Field)
				}
			case sigilChordPrefix:
				if !isPrefix {
					t.Errorf("sigil %q (%s) is not a chord prefix", trigger, definition.Field)
				}
				if isRoot {
					t.Errorf("sigil %q (%s) is a chord prefix and a root key at once", trigger, definition.Field)
				}
			case sigilPickerOnly:
				if isRoot || isPrefix {
					t.Errorf(
						"sigil %q (%s) is documented as picker-only but the keymap binds it",
						trigger,
						definition.Field,
					)
				}
			case sigilFacetOnly:
				if isRoot || isPrefix {
					t.Errorf(
						"sigil %q (%s) is documented as facet-only but the keymap binds it",
						trigger,
						definition.Field,
					)
				}
				if _, _, resolution := registeredSimplifiedOperationRegistry(t).InputBindingForSequence(
					operation.InputStrategySimplified,
					[]string{"f", trigger},
					simplifiedOperationContext(),
				); resolution != operation.ChordTerminal {
					t.Errorf("facet-only sigil %q has no f%s chord", trigger, trigger)
				}
			case sigilSearchOnly:
				if definition.Surfaces&^textgrammar.SurfaceSearch != 0 {
					t.Errorf(
						"sigil %q (%s) is documented as search-only but reaches a mutation surface",
						trigger,
						definition.Field,
					)
				}
			}
		}
	}
	for trigger := range grammarSigilRoutes {
		if !declared[trigger] {
			t.Errorf("keyboard route names %q, which the text grammar no longer declares", trigger)
		}
	}

	// A retired sigil is recognized only to produce a migration error, so it must
	// never reach the advertised set the keymap answers to.
	for _, field := range []textgrammar.Field{textgrammar.FieldTaskType} {
		if definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, field); ok {
			t.Errorf("retired field %s is advertised as %q", field, definition.Trigger)
		}
	}
}

// TestReviewHasNoDirectKey holds the one destination the grammar explicitly
// rejects. $review is refused because a destination is a singleton and review
// must coexist with a lifecycle state on one line, so binding a review setter to
// a root key would teach a route the parser refuses to honor. Review is written
// through the =review sigil on all four surfaces and narrowed through the f=
// facet; neither is a root key.
func TestReviewHasNoDirectKey(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	if _, exists := registry.Descriptor(operation.OperationID("tui.task.toggle-review")); exists {
		t.Error("the retired review setter is registered again")
	}
	for _, entry := range taskOperationDescriptorManifest() {
		if strings.Contains(string(entry.ID), "review") {
			t.Errorf("task operation manifest lists review setter %s", entry.ID)
		}
	}
	strategy := SimplifiedInputStrategy()
	for _, binding := range strategy.Bindings {
		if !strings.Contains(string(binding.OperationID), "review") {
			continue
		}
		if binding.OperationID != operation.OperationFilterReview {
			t.Errorf("%s is bound to %v; review has no setter key", binding.OperationID, binding.Paths())
		}
	}
	binding, _, resolution := registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"f", "="},
		simplifiedOperationContext(),
	)
	if resolution != operation.ChordTerminal || binding.OperationID != operation.OperationFilterReview {
		t.Fatalf("f= = %s (%v), want %s terminal", binding.OperationID, resolution, operation.OperationFilterReview)
	}
}

// TestFilterAndSortSecondKeysUseSigilsWhereOneExists keeps the second key of a
// chord spelled like the field it narrows. A filter over a sigil-carrying field
// answers to that sigil, and the mnemonic initials those chords used to take are
// gone rather than kept as aliases.
//
// The context facet is the one stated exception. Its sigil is ##, and f## is
// structurally impossible: (*inputChordNode).insert builds a prefix-free trie
// and f# is already terminal on the tag facet, so a second # could never be
// reached. Context therefore takes the letter fc — which is also the key the f#
// picker already bound to jump to its CONTEXTS section — and completion age,
// whose field carries no sigil at all, moved to fa to make room.
func TestFilterAndSortSecondKeysUseSigilsWhereOneExists(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	for _, testCase := range []struct {
		sequence []string
		want     operation.ChordResolution
	}{
		{sequence: []string{"f", "!"}, want: operation.ChordPending},
		{sequence: []string{"f", "!", "1"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "#"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "$"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "@"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "*"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "="}, want: operation.ChordTerminal},
		{sequence: []string{"f", "s"}, want: operation.ChordTerminal},
		{sequence: []string{"F", "#"}, want: operation.ChordTerminal},
		{sequence: []string{"o", "!"}, want: operation.ChordTerminal},
		{sequence: []string{"o", "@"}, want: operation.ChordTerminal},
		{sequence: []string{"f", "p"}, want: operation.ChordNoMatch},
		{sequence: []string{"f", "r"}, want: operation.ChordNoMatch},
		{sequence: []string{"f", "t"}, want: operation.ChordNoMatch},
		{sequence: []string{"f", "d"}, want: operation.ChordNoMatch},
		{sequence: []string{"o", "p"}, want: operation.ChordNoMatch},
		{sequence: []string{"F", "t"}, want: operation.ChordNoMatch},
	} {
		t.Run(strings.Join(testCase.sequence, ""), func(t *testing.T) {
			_, _, resolution := registry.InputBindingForSequence(
				operation.InputStrategySimplified,
				testCase.sequence,
				ctx,
			)
			if resolution != testCase.want {
				t.Fatalf(
					"%s = %v, want %v",
					strings.Join(testCase.sequence, ""),
					resolution,
					testCase.want,
				)
			}
		})
	}

	// The stated exception, asserted rather than only described: fc is terminal
	// and it is the context facet that owns it, not the completion age that used
	// to. fa is checked alongside it so the two cannot silently swap back.
	for _, testCase := range []struct {
		sequence []string
		want     operation.OperationID
	}{
		{sequence: []string{"f", "c"}, want: operation.OperationFilterContexts},
		{sequence: []string{"f", "a"}, want: operation.OperationFilterCompletionAge},
	} {
		binding, _, resolution := registry.InputBindingForSequence(
			operation.InputStrategySimplified,
			testCase.sequence,
			ctx,
		)
		if resolution != operation.ChordTerminal || binding.OperationID != testCase.want {
			t.Fatalf(
				"%s = %s (%v), want %s terminal",
				strings.Join(testCase.sequence, ""),
				binding.OperationID,
				resolution,
				testCase.want,
			)
		}
	}
}
