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
	// sigilRenamedChordPrefix is a trigger whose keyboard route is a chord prefix
	// spelled with a different character. It is one route rather than a variant of
	// sigilChordPrefix because the correspondence it breaks has to be named where
	// the correspondence is asserted: grammarSigilKeyboardSpelling says which key
	// carries it, and the trigger itself must reach no key at all.
	sigilRenamedChordPrefix
	// sigilSearchOnly is a trigger that exists nowhere but the search box. It has
	// no keyboard route at all, and unlike a picker-only sigil the keymap is free
	// to bind the same character to an unrelated act: a query is typed into an
	// input, where root keys do not resolve. ? is exactly that — it asks for help
	// on a row and names a computed property in a query.
	sigilSearchOnly
)

// grammarSigilRoutes pairs every advertised text-grammar trigger with the route
// the Outliner gives it. The ten keyed sigils are the correspondence itself:
// the character that writes a field in a capture line is the key that edits it
// on a row. `$` is the one advertised trigger that breaks it, which is why the
// break is a route of its own rather than an omission from the table.
// The four picker-only triggers are named rather than omitted so a
// new sigil forces a deliberate decision instead of silently passing, and a
// renamed one breaks the keymap loudly.
var grammarSigilRoutes = map[string]sigilKeyRoute{
	"!": sigilChordPrefix,
	"#": sigilRootKey,
	"@": sigilRootKey,
	// The destination sigil is the one advertised trigger the keyboard spells
	// differently. `$destination` still writes the ten lifecycle and planning
	// destinations on every mutation surface, but the chord that applies them on
	// a row is `w`: it is the most-pressed chord of the daily loop, and an
	// unshifted letter is worth more there than the correspondence is.
	"$": sigilRenamedChordPrefix,
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
	// Review is a stored flag like the pin, so the character that writes it in a
	// capture line is the key that toggles it on a row. The f= facet spelled with
	// the same character narrows by what this writes; the two are not in tension,
	// because one is a chord under f and the other a root key.
	"=":  sigilRootKey,
	"-%": sigilSearchOnly,
	// A derived property is computed from the task rather than stored on it, so
	// there is nothing for a key to write and both spellings stay in the query.
	"?":  sigilSearchOnly,
	"-?": sigilSearchOnly,
}

// grammarSigilKeyboardSpelling names the key that carries a renamed trigger.
// Every sigilRenamedChordPrefix entry needs one, so a rename is a line of data
// rather than prose: the keymap has to declare the named prefix, and the trigger
// itself has to reach neither a root key nor a prefix of its own.
var grammarSigilKeyboardSpelling = map[string]string{
	"$": "w",
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
			case sigilRenamedChordPrefix:
				spelling, named := grammarSigilKeyboardSpelling[trigger]
				if !named {
					t.Errorf("sigil %q (%s) is documented as renamed but names no keyboard spelling", trigger, definition.Field)
					continue
				}
				if _, bound := prefixes[spelling]; !bound {
					t.Errorf("sigil %q (%s) is documented as reaching %q, which is not a chord prefix",
						trigger, definition.Field, spelling)
				}
				if isRoot || isPrefix {
					t.Errorf(
						"sigil %q (%s) reaches the keyboard as %q, so the character itself must stay unbound",
						trigger,
						definition.Field,
						spelling,
					)
				}
			case sigilPickerOnly:
				if isRoot || isPrefix {
					t.Errorf(
						"sigil %q (%s) is documented as picker-only but the keymap binds it",
						trigger,
						definition.Field,
					)
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
	for trigger := range grammarSigilKeyboardSpelling {
		if grammarSigilRoutes[trigger] != sigilRenamedChordPrefix {
			t.Errorf("%q names a keyboard spelling but is not documented as a renamed sigil", trigger)
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

// TestReviewKeyWritesTheFlagAndKeepsItsFacet holds the whole keyboard surface of
// the review axis. `=` toggles the stored flag on the frozen task target the way
// `*` toggles the pin, `f=` still narrows by it, and `R` acknowledges one
// candidate and moves on — three operations, and each owns exactly one of them:
// a setter that writes, a facet that reads, and an acknowledgement that answers.
// `=` gains neither half of `R`, which is the whole point of keeping the
// acknowledgement its own descriptor rather than a mode of the toggle. The
// destination the grammar rejects stays rejected — $review is refused because a
// destination is a singleton and review must coexist with a lifecycle state on
// one line — so review is still not reachable through the workflow chord the `$`
// sigil's destinations are applied by.
func TestReviewKeyWritesTheFlagAndKeepsItsFacet(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	for _, id := range []operation.OperationID{
		operation.OperationTaskToggleReview,
		operation.OperationTaskMarkReviewed,
	} {
		if _, exists := registry.Descriptor(id); !exists {
			t.Fatalf("%s is not registered", id)
		}
	}
	manifested := map[operation.OperationID]bool{}
	for _, entry := range taskOperationDescriptorManifest() {
		if !strings.Contains(string(entry.ID), "review") {
			continue
		}
		manifested[entry.ID] = true
		switch entry.ID {
		case operation.OperationTaskToggleReview:
			if !entry.Bulk {
				t.Error("the review setter must carry every selected task, not just the cursor")
			}
			if !entry.DirectOnly {
				t.Error("the review setter has a direct key, so it must stay out of the task Space sheet")
			}
		case operation.OperationTaskMarkReviewed:
			// The acknowledgement is one task's act: it advances the cursor to the
			// next candidate, which a selection has no meaning for, so refusing under
			// one is what Bulk false buys.
			if entry.Bulk {
				t.Error("the acknowledgement must refuse a visual selection rather than sweep it")
			}
			if !entry.DirectOnly {
				t.Error("the acknowledgement has a direct key, so it must stay out of the task Space sheet")
			}
		default:
			t.Errorf("task operation manifest lists an unexpected review operation %s", entry.ID)
		}
	}
	for _, id := range []operation.OperationID{
		operation.OperationTaskToggleReview,
		operation.OperationTaskMarkReviewed,
	} {
		if !manifested[id] {
			t.Fatalf("the task operation manifest does not list %s", id)
		}
	}

	strategy := SimplifiedInputStrategy()
	keys := map[operation.OperationID][]string{}
	for _, binding := range strategy.Bindings {
		if !strings.Contains(string(binding.OperationID), "review") {
			continue
		}
		switch binding.OperationID {
		case operation.OperationTaskToggleReview, operation.OperationTaskMarkReviewed:
			keys[binding.OperationID] = append(keys[binding.OperationID], binding.Keys...)
		case operation.OperationFilterReview:
		default:
			t.Errorf("%s is bound to %v; review is one setter, one facet, and one acknowledgement",
				binding.OperationID, binding.Paths())
		}
	}
	for id, want := range map[operation.OperationID]string{
		operation.OperationTaskToggleReview: "=",
		operation.OperationTaskMarkReviewed: "R",
	} {
		if got := keys[id]; len(got) != 1 || got[0] != want {
			t.Errorf("%s is bound to %v, want the %s root key alone", id, got, want)
		}
	}

	ctx := simplifiedOperationContext()
	binding, _, resolution := registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"f", "="},
		ctx,
	)
	if resolution != operation.ChordTerminal || binding.OperationID != operation.OperationFilterReview {
		t.Fatalf("f= = %s (%v), want %s terminal", binding.OperationID, resolution, operation.OperationFilterReview)
	}
	if _, _, resolution := registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"w", "="},
		ctx,
	); resolution != operation.ChordNoMatch {
		t.Fatalf("w= resolves to %v; $review stays a refused destination", resolution)
	}
}

// TestFilterAndSortSecondKeysUseSigilsWhereOneExists keeps the second key of a
// chord spelled like the field it narrows. A filter over a sigil-carrying field
// answers to that sigil, and the mnemonic initials those chords used to take are
// gone rather than kept as aliases.
//
// There are two stated exceptions. The context facet's sigil is ##, and f## is
// structurally impossible: (*inputChordNode).insert builds a prefix-free trie
// and f# is already terminal on the tag facet, so a second # could never be
// reached. Context therefore takes the letter fc — which is also the key the f#
// picker already bound to jump to its CONTEXTS section — and completion age,
// whose field carries no sigil at all, moved to fa to make room.
//
// The states facet is the other. Its field is written $destination, but the
// chord that applies a destination on a row is w rather than $, and a facet
// that narrowed by one spelling while the setter answered to another would be
// two vocabularies over one axis. It follows the keyboard: fw narrows, f$ is
// retired outright rather than kept as an alias.
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
		{sequence: []string{"f", "$"}, want: operation.ChordNoMatch},
		{sequence: []string{"f", "w"}, want: operation.ChordTerminal},
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

	// The stated exceptions, asserted rather than only described: fc is terminal
	// and it is the context facet that owns it, not the completion age that used
	// to. fa is checked alongside it so the two cannot silently swap back, and fw
	// is checked so the states facet cannot drift off the workflow chord's letter.
	for _, testCase := range []struct {
		sequence []string
		want     operation.OperationID
	}{
		{sequence: []string{"f", "c"}, want: operation.OperationFilterContexts},
		{sequence: []string{"f", "a"}, want: operation.OperationFilterCompletionAge},
		{sequence: []string{"f", "w"}, want: operation.OperationFilterStates},
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
