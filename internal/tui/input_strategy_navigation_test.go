package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func TestProductionInputStrategyDefaultsToSimplified(t *testing.T) {
	runtime := newTestConfig()
	app, err := NewAppModel(runtime, nil, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if app.operationRegistry == nil || app.operationExecutor == nil {
		t.Fatal("default operation system is not initialized")
	}
	if app.crossOutliner == nil || app.crossOutliner.operationRegistry != app.operationRegistry {
		t.Fatal("root Outliner is not wired to the operation registry")
	}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ':', Text: ":"})
	if !handled || cmd != nil {
		t.Fatalf("removed Finder key = handled %v cmd %v; want consumed no-op", handled, cmd)
	}

	binding, _, resolution := app.operationRegistry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"g", "g"},
		app.operationCommandContext(),
	)
	if resolution != ChordTerminal || binding.OperationID != operationCoreCursorTop {
		t.Fatalf("gg resolved to %s (resolution %v), want %s", binding.OperationID, resolution, operationCoreCursorTop)
	}

	handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'F', Text: "F"})
	if !handled || cmd != nil {
		t.Fatalf("retired legacy key F = handled %v cmd %v; want consumed no-op", handled, cmd)
	}
}

// TestGoToChordResolvesTopJumpAndEveryMode covers the bindings that declare
// direct keys and a chord sequence together, and keeps the g subtree prefix-free
// with cursor-top and task-project navigation beside the seven mode jumps. Each
// mode jump answers to its chord and to its Tab-cycle digit, and both spellings
// are projected from the one binding, so the composed DisplayKey is asserted
// here rather than left to the help renderer to discover.
func TestGoToChordResolvesTopJumpAndEveryMode(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	paths := map[string][]string{
		"gg": {"g", "g"},
		"ga": {"g", "a"},
		"gi": {"g", "i"},
		"gt": {"g", "t"},
		"gn": {"g", "n"},
		"gb": {"g", "b"},
		"g@": {"g", "@"},
		"gr": {"g", "r"},
		// The digit aliases resolve to the same seven bindings, so the second
		// spelling is audited beside the first rather than in a test of its own.
		"1": {"1"},
		"2": {"2"},
		"3": {"3"},
		"4": {"4"},
		"5": {"5"},
		"6": {"6"},
		"7": {"7"},
		// 0 is the eighth digit and the one that names no mode.
		"0": {"0"},
	}
	want := map[string]OperationID{
		"gg": operationCoreCursorTop,
		"ga": operationCoreGotoAll,
		"gi": operationCoreGotoInbox,
		"gt": operationCoreGotoToday,
		"gn": operationCoreGotoNext,
		"gb": operationCoreGotoBacklog,
		"g@": operationCoreGotoDue,
		"gr": operationCoreGotoRecent,
		"1":  operationCoreGotoAll,
		"2":  operationCoreGotoToday,
		"3":  operationCoreGotoInbox,
		"4":  operationCoreGotoNext,
		"5":  operationCoreGotoBacklog,
		"6":  operationCoreGotoDue,
		"7":  operationCoreGotoRecent,
		"0":  operationCoreResetToLaunchView,
	}
	for spelling, path := range paths {
		binding, _, resolution := registry.InputBindingForSequence(InputStrategySimplified, path, ctx)
		if resolution != ChordTerminal {
			t.Errorf("%s resolution = %v, want ChordTerminal", spelling, resolution)
			continue
		}
		if binding.OperationID != want[spelling] {
			t.Errorf("%s resolved to %s, want %s", spelling, binding.OperationID, want[spelling])
		}
	}
	taskContext := ctx
	taskContext.Entity = EntityContext{ProjectID: "project", TaskID: "task"}
	binding, _, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"g", "p"},
		taskContext,
	)
	if resolution != ChordTerminal || binding.OperationID != operationCoreGotoTaskProject {
		t.Errorf("gp resolved to %s (resolution %v), want %s", binding.OperationID, resolution, operationCoreGotoTaskProject)
	}

	if _, _, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"g"},
		ctx,
	); resolution != ChordPending {
		t.Errorf("bare g resolution = %v, want ChordPending", resolution)
	}

	binding, _, ok := registry.InputBindingForKey(InputStrategySimplified, "home", ctx)
	if !ok || binding.OperationID != operationCoreCursorTop {
		t.Errorf("Home resolved to %s (found %v), want %s", binding.OperationID, ok, operationCoreCursorTop)
	}

	if label, ok := registry.ChordGroupLabelForPrefix(
		InputStrategySimplified,
		[]string{"g", "q"},
	); !ok || label != "go to" {
		t.Errorf("abandoned g chord label = %q (found %v), want %q", label, ok, "go to")
	}

	// Both spellings reach ? help and the footer for free, because the projection
	// reads the binding's one DisplayKey rather than composing a second one.
	for operationID, want := range map[OperationID]string{
		operationCoreGotoAll:           "ga/1",
		operationCoreGotoToday:         "gt/2",
		operationCoreGotoInbox:         "gi/3",
		operationCoreGotoNext:          "gn/4",
		operationCoreGotoBacklog:       "gb/5",
		operationCoreGotoDue:           "g@/6",
		operationCoreGotoRecent:        "gr/7",
		operationCoreResetToLaunchView: "0",
	} {
		if got := registry.InputShortcutForOperation(InputStrategySimplified, operationID); got != want {
			t.Errorf("%s display key = %q, want %q", operationID, got, want)
		}
	}
}

// TestScrollChordResolvesEveryAnchor keeps z a chord prefix rather than the
// retired snooze key, with the three scroll anchors under it. Folding left the
// prefix entirely for the s/p/t family, so the group is named "scroll" again.
func TestScrollChordResolvesEveryAnchor(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	for spelling, want := range map[string]OperationID{
		"zt": operationCoreScrollCursorTop,
		"zz": operationCoreScrollCursorMiddle,
		"zb": operationCoreScrollCursorBottom,
	} {
		path := []string{"z", spelling[1:]}
		binding, _, resolution := registry.InputBindingForSequence(InputStrategySimplified, path, ctx)
		if resolution != ChordTerminal {
			t.Errorf("%s resolution = %v, want ChordTerminal", spelling, resolution)
			continue
		}
		if binding.OperationID != want {
			t.Errorf("%s resolved to %s, want %s", spelling, binding.OperationID, want)
		}
	}

	if _, _, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"z"},
		ctx,
	); resolution != ChordPending {
		t.Errorf("bare z resolution = %v, want ChordPending", resolution)
	}

	if label, ok := registry.ChordGroupLabelForPrefix(
		InputStrategySimplified,
		[]string{"z", "q"},
	); !ok || label != "scroll" {
		t.Errorf("abandoned z chord label = %q (found %v), want %q", label, ok, "scroll")
	}
}

func TestRemovedFinderKeyDoesNotEscapeChildRoutes(t *testing.T) {
	runtime := newTestConfig()
	app, err := NewAppModel(runtime, nil, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	app.pushView(presentation.ViewProjectHub)

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: ':', Text: ":"})
	if !handled || cmd != nil {
		t.Fatalf("child-route Finder key = handled %v cmd %v; want consumed no-op", handled, cmd)
	}
}
