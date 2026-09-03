package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestNotesDirectGrammarMatchesTheWorkspaceManager(t *testing.T) {
	view := &NotesViewModel{}
	// home and G are the physical keys behind the two end rows the manager's own
	// help names as gg/Home and G/End; the help spelling is display only, so
	// HandledKeys still reports the keys the keymap actually carries.
	want := []string{
		"j", "k", "down", "up", "ctrl+f", "ctrl+b", "home", "G",
		"enter", "a", "e", "%", "/", "p", "space", "u", "ctrl+r", "y", "Y",
	}
	if got := view.HandledKeys(); !slices.Equal(got, want) {
		t.Fatalf("Notes direct keys = %v, want %v", got, want)
	}
	// n, v, s, r, m and x are the retired spellings the grammar must now leave
	// silent; ctrl+n, t and ctrl+o name vocabulary Notes never regained.
	for _, excluded := range []string{
		"n", "v", "s", "r", "m", "x", "ctrl+n", "t", "ctrl+o",
	} {
		if slices.Contains(view.HandledKeys(), excluded) {
			t.Errorf("Notes direct grammar exposes excluded key %q", excluded)
		}
	}
}

func TestFinalPriorityChordUsesBangPrefix(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	got := make(map[string]any)
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operationTaskSetPriority ||
			len(binding.Sequence) != 2 ||
			binding.Sequence[0] != "!" {
			continue
		}
		got[binding.Sequence[1]] = binding.PresetValue
	}
	want := map[string]any{"0": 0, "1": 1, "2": 2, "3": 3}
	if len(got) != len(want) {
		t.Fatalf("priority chord terminals = %v, want %v", got, want)
	}
	for key, priority := range want {
		if got[key] != priority {
			t.Errorf("!%s = %v, want %v", key, got[key], priority)
		}
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operationTaskSetPriority &&
			len(binding.Sequence) > 0 &&
			binding.Sequence[0] == "p" {
			t.Errorf("retired priority prefix remains registered: %v", binding.Sequence)
		}
	}
}

func TestFinalStateChordOwnsEveryStateAndPlanningAction(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	type terminal struct {
		operation operation.OperationID
		value     any
		hasValue  bool
		display   string
	}
	got := make(map[string]terminal)
	gotOrder := make([]string, 0, 10)
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) != 2 || binding.Sequence[0] != "$" {
			continue
		}
		key := binding.Sequence[1]
		gotOrder = append(gotOrder, key)
		got[key] = terminal{
			operation: binding.OperationID,
			value:     binding.PresetValue,
			hasValue:  binding.HasPresetValue,
			display:   binding.DisplayKey,
		}
	}
	const stateSpelling = "$i/g/w/p/m/d/c"
	want := map[string]terminal{
		"i": {operation: operationTaskSetState, value: models.StateInbox, hasValue: true, display: stateSpelling},
		"b": {operation: operation.OperationCoreBacklog, display: "$b"},
		"n": {operation: operation.OperationCoreNext, display: "$n"},
		"t": {operation: operation.OperationCoreToday, display: "$t"},
		"g": {operation: operationTaskSetState, value: models.StateDoing, hasValue: true, display: stateSpelling},
		"w": {operation: operationTaskSetState, value: models.StateWaiting, hasValue: true, display: stateSpelling},
		"p": {operation: operationTaskSetState, value: models.StatePaused, hasValue: true, display: stateSpelling},
		"m": {operation: operationTaskSetState, value: models.StateSomeday, hasValue: true, display: stateSpelling},
		"d": {operation: operationTaskSetState, value: models.StateDone, hasValue: true, display: stateSpelling},
		"c": {operation: operationTaskSetState, value: models.StateCanceled, hasValue: true, display: stateSpelling},
	}
	if len(got) != len(want) {
		t.Fatalf("state chord terminals = %v, want %v", got, want)
	}
	if wantOrder := []string{"i", "b", "n", "t", "g", "w", "p", "m", "d", "c"}; !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("state chord terminal order = %v, want %v", gotOrder, wantOrder)
	}
	for key, terminal := range want {
		if !reflect.DeepEqual(got[key], terminal) {
			t.Errorf("$%s = %+v, want %+v", key, got[key], terminal)
		}
	}
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) > 0 && binding.Sequence[0] == "," {
			t.Errorf("retired comma state prefix remains registered: %v", binding.Sequence)
		}
	}
}

func TestDirectTaskGrammarRemainsDocumentedWithoutACursorTarget(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	items := registry.HelpItemsForInputStrategy(
		operation.InputStrategySimplified,
		simplifiedOperationContext(),
	)
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.Key] = true
	}
	for _, key := range []string{"A", "@", "#", "*", "[", "]", ">", "<", "~", "&", ";", "y", "Y", "Ctrl+Y"} {
		if !keys[key] {
			t.Errorf("target-free help omits documented direct key %q", key)
		}
	}
}
