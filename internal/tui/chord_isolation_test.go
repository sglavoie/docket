package tui

import (
	"slices"
	"strings"
	"testing"
)

// TestChordPrefixesNeverFallThroughToRootOperations presses every root key of
// the grammar after every chord prefix. A second key must resolve inside the
// chord it continues or resolve to nothing at all; a key that leaked back to the
// root grammar would silently run an operation the which-key row never offered,
// which is exactly what adding a second uppercase filter root risks.
func TestChordPrefixesNeverFallThroughToRootOperations(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified strategy was not registered")
	}
	ctx := simplifiedOperationContext()

	rootKeys := make([]string, 0)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.paths() {
			if !slices.Contains(rootKeys, path[0]) {
				rootKeys = append(rootKeys, path[0])
			}
		}
	}
	if len(rootKeys) == 0 {
		t.Fatal("the grammar declares no root keys; the invariant is not auditing anything")
	}

	for _, group := range strategy.ChordGroups {
		for _, key := range rootKeys {
			sequence := append(slices.Clone(group.Sequence), key)
			binding, _, resolution := registry.InputBindingForSequence(
				InputStrategySimplified,
				sequence,
				ctx,
			)
			switch resolution {
			case ChordNoMatch, ChordPending:
				continue
			case ChordTerminal:
			}
			if len(binding.Sequence) < len(sequence) ||
				!slices.Equal(binding.Sequence[:len(group.Sequence)], group.Sequence) {
				t.Errorf(
					"%s resolves %s, which is not inside the %s chord",
					strings.Join(sequence, ""),
					binding.OperationID,
					strings.Join(group.Sequence, ""),
				)
			}
		}
	}
}

// TestAbandonedChordNamesThePathItLeft keeps a missed second key explainable.
// The which-key row promised a chord, so resolving to nothing has to say which
// chord was abandoned rather than leaving the screen unchanged.
func TestAbandonedChordNamesThePathItLeft(t *testing.T) {
	app := headerTargetAuditApp(t)
	for _, testCase := range []struct {
		sequence []string
		want     string
	}{
		{sequence: []string{"$", "q"}, want: "$q is not a workflow action"},
		{sequence: []string{"f", "q"}, want: "fq is not a filter action"},
		{sequence: []string{"F", "q"}, want: "Fq is not a project filter action"},
	} {
		t.Run(strings.Join(testCase.sequence, ""), func(t *testing.T) {
			app.statusMessage = ""
			for _, name := range testCase.sequence {
				key, err := parseUITestKey(name)
				if err != nil {
					t.Fatalf("parse %q: %v", name, err)
				}
				if handled, _ := app.handleSimplifiedInputKey(key); !handled {
					t.Fatalf("key %q was not consumed by the Outliner grammar", name)
				}
			}
			if app.statusMessage != testCase.want {
				t.Fatalf("chord miss status = %q, want %q", app.statusMessage, testCase.want)
			}
		})
	}
}

// TestRetiredRootKeysStaySilentlyUnbound keeps the chord-miss status scoped to
// abandoned chords. A root key never promised anything, so explaining it would
// turn every stray keystroke into a message. The table holds the keys the sigil
// grammar retired beside a key that was never bound at all, because a retired
// key has to be as silent as an unknown one: a shim that answered w or P would
// keep two vocabularies alive.
func TestRetiredRootKeysStaySilentlyUnbound(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	app := headerTargetAuditApp(t)
	for _, name := range []string{
		"w", "P", "d", "b", "t", "c", "m", ":", "C", "R", ",", "q",
	} {
		t.Run(name, func(t *testing.T) {
			for _, binding := range strategy.Bindings {
				for _, path := range binding.paths() {
					if len(path) == 1 && path[0] == name {
						t.Fatalf("retired root key %q is bound to %s", name, binding.OperationID)
					}
				}
			}
			app.statusMessage = ""
			key, err := parseUITestKey(name)
			if err != nil {
				t.Fatalf("parse %q: %v", name, err)
			}
			if handled, cmd := app.handleSimplifiedInputKey(key); !handled || cmd != nil {
				t.Fatalf("unbound root key %q = handled %v, cmd %v; want consumed no-op", name, handled, cmd)
			}
			if app.statusMessage != "" {
				t.Fatalf("unbound root key %q set status %q", name, app.statusMessage)
			}
		})
	}
}
