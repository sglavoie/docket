package tui

import (
	"slices"
	"strings"
	"testing"
)

func TestInputStrategyRegistrationRejectsCollisionsAndPrefixes(t *testing.T) {
	newRegistry := func(t *testing.T) *OperationRegistry {
		t.Helper()
		registry := NewOperationRegistry()
		for _, descriptor := range coreInputOperationDescriptors()[:2] {
			if err := registry.Register(descriptor); err != nil {
				t.Fatal(err)
			}
		}
		return registry
	}

	t.Run("collision", func(t *testing.T) {
		registry := newRegistry(t)
		err := registry.RegisterInputStrategy(InputStrategy{
			ID: "collision",
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"a"}},
				{OperationID: operationCoreEdit, Keys: []string{"a"}},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "collision") {
			t.Fatalf("error = %v, want collision rejection", err)
		}
	})

	t.Run("prefix", func(t *testing.T) {
		registry := newRegistry(t)
		err := registry.RegisterInputStrategy(InputStrategy{
			ID: "prefix",
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"gg"}},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "direct key event") {
			t.Fatalf("error = %v, want prefix rejection", err)
		}
	})
}

const chordTestSetPriority OperationID = "test.task.set-priority"

// chordTestRegistry adds one value-taking operation to the core descriptors so
// preset-valued chord bindings can be exercised without the production catalog.
func chordTestRegistry(t *testing.T) *OperationRegistry {
	t.Helper()
	registry := NewOperationRegistry()
	for _, descriptor := range coreInputOperationDescriptors() {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.Register(OperationDescriptor{
		ID:                   chordTestSetPriority,
		Label:                "Set Priority",
		Category:             OperationCategoryTask,
		TargetSource:         OperationTargetCurrentTask,
		TargetCardinality:    TargetCardinalityOne,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Applicability:        coreOutlinerApplicability,
		Risk:                 OperationRiskMutation,
		ValueProviderID:      ValueProviderPriority,
	}); err != nil {
		t.Fatalf("Register(%s): %v", chordTestSetPriority, err)
	}
	return registry
}

func chordTestStrategy() InputStrategy {
	return InputStrategy{
		ID: "chords",
		Bindings: []OperationKeyBinding{
			{OperationID: operationCoreCapture, Keys: []string{"a"}, FooterRank: 1, FooterLabel: "capture"},
			{
				OperationID:    chordTestSetPriority,
				Sequence:       []string{"p", "1"},
				DisplayKey:     "p1-2",
				HintLabel:      "critical",
				PresetValue:    1,
				HasPresetValue: true,
			},
			{
				OperationID:    chordTestSetPriority,
				Sequence:       []string{"p", "2"},
				DisplayKey:     "p1-2",
				HintLabel:      "high",
				PresetValue:    2,
				HasPresetValue: true,
			},
			{OperationID: operationCoreHelp, Sequence: []string{",", "h"}, HintLabel: "help"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"p"}, HintLabel: "priority", FooterRank: 2, FooterLabel: "priority"},
			{Sequence: []string{","}, HintLabel: "state"},
		},
	}
}

func TestChordSequencesResolveAndProjectThroughOneStrategy(t *testing.T) {
	registry := chordTestRegistry(t)
	if err := registry.RegisterInputStrategy(chordTestStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	ctx := simplifiedOperationContext()

	if _, _, resolution := registry.InputBindingForSequence("chords", []string{"p"}, ctx); resolution != ChordPending {
		t.Fatalf("prefix p resolution = %v, want pending", resolution)
	}
	binding, descriptor, resolution := registry.InputBindingForSequence("chords", []string{"p", "2"}, ctx)
	if resolution != ChordTerminal {
		t.Fatalf("chord p2 resolution = %v, want terminal", resolution)
	}
	if descriptor.ID != chordTestSetPriority {
		t.Errorf("chord p2 resolved %s, want %s", descriptor.ID, chordTestSetPriority)
	}
	if !binding.HasPresetValue || binding.PresetValue != 2 {
		t.Errorf("chord p2 preset = (%v, %v), want (2, true)", binding.PresetValue, binding.HasPresetValue)
	}
	if _, _, resolution := registry.InputBindingForSequence("chords", []string{"p", "9"}, ctx); resolution != ChordNoMatch {
		t.Errorf("unbound chord p9 resolution = %v, want no match", resolution)
	}
	if _, _, ok := registry.InputBindingForKey("chords", "p", ctx); ok {
		t.Error("a chord prefix resolved as a direct key")
	}
	if _, _, ok := registry.InputBindingForKey("chords", "a", ctx); !ok {
		t.Error("direct key a did not resolve as a length-1 sequence")
	}
	// The subtree is unreachable outside the Outliner, so its prefix must fail
	// closed instead of trapping input in a chord that can never complete.
	if _, _, resolution := registry.InputBindingForSequence(
		"chords",
		[]string{"p"},
		PresentationCommandContext(ViewProjectHub),
	); resolution != ChordNoMatch {
		t.Error("an inapplicable chord subtree still accepted its prefix")
	}

	handled := registry.HandledKeysForInputStrategy("chords")
	for _, key := range []string{"a", "p", "1", "2", ",", "h"} {
		if !slices.Contains(handled, key) {
			t.Errorf("handled keys %v omit chord key %q", handled, key)
		}
	}

	if got := registry.InputShortcutForOperation("chords", chordTestSetPriority); got != "p1-2" {
		t.Errorf("shortcut = %q, want the shared preset spelling %q", got, "p1-2")
	}
	help := registry.HelpItemsForInputStrategy("chords", ctx)
	if len(help) != 3 {
		t.Fatalf("help items = %d, want one per operation (3)", len(help))
	}
	if help[1].Key != "p1-2" {
		t.Errorf("preset family help key = %q, want %q", help[1].Key, "p1-2")
	}
	if help[2].Key != ",h" {
		t.Errorf("chord help key = %q, want the composed sequence %q", help[2].Key, ",h")
	}

	hints := registry.ChordHintsForPrefix("chords", []string{"p"}, ctx)
	want := []ChordHint{{Key: "1", Label: "critical"}, {Key: "2", Label: "high"}}
	if !slices.Equal(hints, want) {
		t.Errorf("which-key hints = %v, want %v", hints, want)
	}
	row := guidanceHintsText(pendingChordGuidanceSpec("p", hints).Hints)
	for _, fragment := range []string{"p…", "1 critical", "2 high"} {
		if !strings.Contains(row, fragment) {
			t.Errorf("which-key row %q omits %q", row, fragment)
		}
	}
	if hints := registry.ChordHintsForPrefix("chords", []string{"p", "1"}, ctx); len(hints) != 0 {
		t.Error("a terminal chord still produced a which-key row")
	}

	footer := guidanceHintsText(registry.GuidanceHintsForInputStrategy("chords", ctx))
	for _, fragment := range []string{"a capture", "p priority"} {
		if !strings.Contains(footer, fragment) {
			t.Errorf("footer %q omits %q", footer, fragment)
		}
	}
}

func TestChordStrategyRegistrationRejectsAmbiguousAndUnlabeledGrammars(t *testing.T) {
	priorityChord := func(sequence []string, preset any) OperationKeyBinding {
		return OperationKeyBinding{
			OperationID:    chordTestSetPriority,
			Sequence:       sequence,
			PresetValue:    preset,
			HasPresetValue: true,
		}
	}
	priorityGroup := OperationChordGroup{Sequence: []string{"p"}, HintLabel: "priority"}

	tests := []struct {
		name     string
		strategy InputStrategy
		want     string
	}{
		{
			name: "direct key shadows a chord prefix",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					{OperationID: operationCoreCapture, Keys: []string{"p"}},
					priorityChord([]string{"p", "1"}, 1),
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "prefix collision",
		},
		{
			name: "chord extends a direct key",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					priorityChord([]string{"p", "1"}, 1),
					{OperationID: operationCoreCapture, Keys: []string{"p"}},
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "prefix collision",
		},
		{
			name: "duplicate chord path",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					priorityChord([]string{"p", "1"}, 1),
					priorityChord([]string{"p", "1"}, 2),
				},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "key collision",
		},
		{
			name: "one-key sequence",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{priorityChord([]string{"p"}, 1)},
			},
			want: "one-key chord sequence",
		},
		{
			name: "repeated operation without presets",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{
					{OperationID: operationCoreCapture, Keys: []string{"a"}},
					{OperationID: operationCoreCapture, Sequence: []string{",", "a"}},
				},
				ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
			},
			want: "more than once",
		},
		{
			name: "preset on an operation that takes no value",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{{
					OperationID:    operationCoreCapture,
					Sequence:       []string{",", "a"},
					PresetValue:    1,
					HasPresetValue: true,
				}},
				ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
			},
			want: "presets a value but takes none",
		},
		{
			name: "preset type the provider cannot accept",
			strategy: InputStrategy{
				Bindings:    []OperationKeyBinding{priorityChord([]string{"p", "1"}, "high")},
				ChordGroups: []OperationChordGroup{priorityGroup},
			},
			want: "does not accept",
		},
		{
			name: "chord prefix without a group label",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{priorityChord([]string{"p", "1"}, 1)},
			},
			want: "has no group label",
		},
		{
			name: "group label for a prefix that does not exist",
			strategy: InputStrategy{
				Bindings: []OperationKeyBinding{{OperationID: operationCoreCapture, Keys: []string{"a"}}},
				ChordGroups: []OperationChordGroup{
					{Sequence: []string{"f"}, HintLabel: "filter"},
				},
			},
			want: "does not label a chord prefix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := chordTestRegistry(t)
			strategy := test.strategy
			strategy.ID = "candidate"
			err := registry.RegisterInputStrategy(strategy)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want one containing %q", err, test.want)
			}
		})
	}
}
