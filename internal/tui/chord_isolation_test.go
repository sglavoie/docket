package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// TestChordPrefixesNeverFallThroughToRootOperations presses every root key of
// the grammar after every chord prefix. A second key must resolve inside the
// chord it continues or resolve to nothing at all; a key that leaked back to the
// root grammar would silently run an operation the which-key row never offered,
// which is exactly what adding a second uppercase filter root risks.
func TestChordPrefixesNeverFallThroughToRootOperations(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified strategy was not registered")
	}
	ctx := simplifiedOperationContext()

	rootKeys := make([]string, 0)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.Paths() {
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
				operation.InputStrategySimplified,
				sequence,
				ctx,
			)
			switch resolution {
			case operation.ChordNoMatch, operation.ChordPending:
				continue
			case operation.ChordTerminal:
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

// TestModifierKeyPressDoesNotAbandonAPendingChord keeps a chord alive across the
// Shift its own second key requires. A terminal that reports every key as an
// escape code sends the modifier as its own press, and consuming that press as
// the chord's second key put every shifted facet — f!, f@, f$, f#, F#, o@, g@ —
// permanently out of reach. It enters at handleGlobalMessage because the shell
// drops the press above handleGlobalKeyPress, where the other chord tests start.
func TestModifierKeyPressDoesNotAbandonAPendingChord(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner

	for _, name := range []string{"f", "leftshift", "!", "2"} {
		key, err := parseUITestKey(name)
		if err != nil {
			t.Fatalf("parse %q: %v", name, err)
		}
		if handled, _ := app.handleGlobalMessage(key); !handled {
			t.Fatalf("key %q was not consumed by the shell", name)
		}
	}

	if app.feedback.message == "fleftshift is not a filter action" {
		t.Fatal("the bare Shift press was consumed as the chord's second key")
	}
	if len(app.operations.Chord) != 0 {
		t.Fatalf("f!2 left pending prefix %v", app.operations.Chord)
	}
	if out.Filter.Priority != 2 {
		t.Fatalf("Priority = %d after f!2 with a Shift press mid-chord, want 2", out.Filter.Priority)
	}
}

// TestModifiedKeysStayKeystrokes keeps the modifier drop narrow. A modifier that
// qualifies another key names a keystroke the grammar binds, so widening the
// drop to anything carrying a KeyMod would unbind Shift+Tab and Shift+Enter.
func TestModifiedKeysStayKeystrokes(t *testing.T) {
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: tea.KeyEnter, Mod: tea.ModShift},
		{Code: tea.KeyLeft, Mod: tea.ModShift},
		{Code: tea.KeyRight, Mod: tea.ModShift},
		{Code: 'a', Text: "a"},
		{Code: '#', Text: "#"},
	} {
		if isModifierOnlyKeyPress(msg) {
			t.Errorf("%s was dropped as a modifier-only key press", msg)
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
			app.feedback.message = ""
			for _, name := range testCase.sequence {
				key, err := parseUITestKey(name)
				if err != nil {
					t.Fatalf("parse %q: %v", name, err)
				}
				if handled, _ := app.handleSimplifiedInputKey(key); !handled {
					t.Fatalf("key %q was not consumed by the Outliner grammar", name)
				}
			}
			if app.feedback.message != testCase.want {
				t.Fatalf("chord miss status = %q, want %q", app.feedback.message, testCase.want)
			}
		})
	}
}

// TestRetiredRootKeysStaySilentlyUnbound keeps the chord-miss status scoped to
// abandoned chords. A root key never promised anything, so explaining it would
// turn every stray keystroke into a message. The table holds the keys the sigil
// grammar retired beside a key that was never bound at all, because a retired
// key has to be as silent as an unknown one: a shim that answered w or m would
// keep two vocabularies alive. t, P, c and C are deliberately absent — the fold
// family took t back for task subtrees, P went back to Select Project, c went
// to Add Checklist Item, and C went to quick Add Comment. b left the table for
// the same reason: Peek took it off Ctrl+Enter, so it is a live root key. R left
// it too: Mark Reviewed took the capital, which the sigil grammar never spelled.
func TestRetiredRootKeysStaySilentlyUnbound(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	app := headerTargetAuditApp(t)
	for _, name := range []string{
		"w", "d", "m", ":", ",", "q",
	} {
		t.Run(name, func(t *testing.T) {
			for _, binding := range strategy.Bindings {
				for _, path := range binding.Paths() {
					if len(path) == 1 && path[0] == name {
						t.Fatalf("retired root key %q is bound to %s", name, binding.OperationID)
					}
				}
			}
			app.feedback.message = ""
			key, err := parseUITestKey(name)
			if err != nil {
				t.Fatalf("parse %q: %v", name, err)
			}
			if handled, cmd := app.handleSimplifiedInputKey(key); !handled || cmd != nil {
				t.Fatalf("unbound root key %q = handled %v, cmd %v; want consumed no-op", name, handled, cmd)
			}
			if app.feedback.message != "" {
				t.Fatalf("unbound root key %q set status %q", name, app.feedback.message)
			}
		})
	}
}
