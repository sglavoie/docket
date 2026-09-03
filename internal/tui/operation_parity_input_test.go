package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestDirectKeysHelpAndHandledKeysDeriveFromTheInputStrategy(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	// Task-scoped keys only resolve over a real target, so the audit runs in the
	// same focused context the user is in when they press them.
	ctx := taskActionParityContext(t, app)
	// The ranked-list toggle is a presentation of a query, so it only exists
	// while one is held. Holding one is the state its key is pressed in; a query
	// on its own still freezes no other axis, so every other binding is audited
	// exactly as before.
	app.crossOutliner.beginSearchProjection()

	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	handled := registry.HandledKeysForInputStrategy(operation.InputStrategySimplified)
	expectedKeys := make([]string, 0, len(handled))
	for _, binding := range strategy.Bindings {
		// A checklist row operation only exists under a checklist cursor, so its
		// keys are audited from the row the user is on when they press them.
		bindingCtx := ctx
		if operation.ChecklistRowGrammar[binding.OperationID] {
			bindingCtx.Entity.HasChecklistCursor = true
		}
		for _, path := range binding.Paths() {
			for _, key := range path {
				if !slices.Contains(expectedKeys, key) {
					expectedKeys = append(expectedKeys, key)
				}
			}
			resolved, descriptor, resolution := registry.InputBindingForSequence(operation.InputStrategySimplified, path, bindingCtx)
			if resolution != operation.ChordTerminal {
				t.Errorf("key path %v does not resolve to an operation in the Outliner context", path)
				continue
			}
			if resolved.OperationID != descriptor.ID {
				t.Errorf("key path %v resolved binding %s but descriptor %s", path, resolved.OperationID, descriptor.ID)
				continue
			}
			// A shared key resolves to whichever sibling the cursor makes
			// applicable, so landing on a co-owner of the same path is the
			// grammar working rather than a collision.
			sharedSibling := binding.SharedKey && sharesInputPath(strategy, path, descriptor.ID)
			if descriptor.ID != binding.OperationID && !sharedSibling {
				t.Errorf("key path %v resolved to %s, want %s", path, descriptor.ID, binding.OperationID)
			}
			// A chord's own prefixes must wait for the next key rather than run.
			for depth := 1; depth < len(path); depth++ {
				_, _, resolution := registry.InputBindingForSequence(operation.InputStrategySimplified, path[:depth], bindingCtx)
				if resolution != operation.ChordPending {
					t.Errorf("chord prefix %v resolution = %v, want pending", path[:depth], resolution)
				}
			}
		}
	}
	if !slices.Equal(handled, expectedKeys) {
		t.Errorf("handled keys = %v, want the strategy keys %v", handled, expectedKeys)
	}

	help := registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx)
	applicable := registry.InputStrategyOperations(operation.InputStrategySimplified, ctx)
	if len(help) != len(applicable) {
		t.Fatalf("help items = %d, want one per applicable binding (%d)", len(help), len(applicable))
	}
	for index, item := range help {
		descriptor := applicable[index]
		if item.Description != descriptor.Label {
			t.Errorf("help row %d describes %q, want the descriptor label %q", index, item.Description, descriptor.Label)
		}
		shortcut := registry.InputShortcutForOperation(operation.InputStrategySimplified, descriptor.ID)
		if item.Key != shortcut {
			t.Errorf("help row %d shows key %q, want the strategy display key %q", index, item.Key, shortcut)
		}
	}
}

func TestOperationParityGuidanceResolvesThroughApplicableInputMetadata(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	ctx := taskActionParityContext(t, app)
	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	hints := registry.GuidanceHintsForInputStrategy(operation.InputStrategySimplified, ctx)
	if len(hints) == 0 {
		t.Fatal("applicable input strategy produced no guidance")
	}
	for _, hint := range hints {
		resolved := false
		for _, binding := range strategy.Bindings {
			if binding.FooterRank <= 0 || binding.DisplayKey != hint.Key {
				continue
			}
			descriptor, exists := registry.Descriptor(binding.OperationID)
			if !exists || !descriptor.Applicable(ctx) {
				continue
			}
			label := binding.FooterLabel
			if label == "" {
				label = strings.ToLower(descriptor.Label)
			}
			if label == hint.Label {
				resolved = true
				break
			}
		}
		if !resolved {
			for _, group := range strategy.ChordGroups {
				if group.FooterRank <= 0 || strings.Join(group.Sequence, "") != hint.Key {
					continue
				}
				label := group.FooterLabel
				if label == "" {
					label = group.HintLabel
				}
				if label == hint.Label && len(registry.ChordHintsForPrefix(operation.InputStrategySimplified, group.Sequence, ctx)) > 0 {
					resolved = true
					break
				}
			}
		}
		if !resolved {
			t.Errorf("guidance hint %q resolves to no applicable operation or chord", chrome.GuidanceHintText(hint))
		}
	}

}

// sharesInputPath reports whether the operation co-owns the key path as a
// declared shared binding rather than having stolen it.
func sharesInputPath(strategy operation.InputStrategy, path []string, id operation.OperationID) bool {
	for _, binding := range strategy.Bindings {
		if binding.OperationID != id || !binding.SharedKey {
			continue
		}
		for _, candidate := range binding.Paths() {
			if slices.Equal(candidate, path) {
				return true
			}
		}
	}
	return false
}

func TestChordBindingsStayPrefixFreeExecutableAndLabeled(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	executor := app.operationExecutor
	ctx := taskActionParityContext(t, app)

	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	paths := make([][]string, 0, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		paths = append(paths, binding.Paths()...)
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists {
			t.Errorf("binding %v references unregistered operation %s", binding.DisplayKey, binding.OperationID)
			continue
		}
		if len(executor.bindings[binding.OperationID]) == 0 {
			t.Errorf("key %s binds unexecutable operation %s", binding.DisplayKey, binding.OperationID)
		}
		if !binding.HasPresetValue {
			continue
		}
		if !descriptor.ValueProviderID.AcceptsPresetValue(binding.PresetValue) {
			t.Errorf(
				"key %s presets %T, which the %s handler does not accept",
				binding.DisplayKey,
				binding.PresetValue,
				descriptor.ValueProviderID,
			)
		}
	}

	for i, left := range paths {
		for j, right := range paths {
			if i == j || len(left) >= len(right) {
				continue
			}
			if slices.Equal(left, right[:len(left)]) {
				t.Errorf("key path %v shadows the longer path %v", left, right)
			}
		}
	}

	for _, group := range strategy.ChordGroups {
		hints := registry.ChordHintsForPrefix(operation.InputStrategySimplified, group.Sequence, ctx)
		if len(hints) == 0 {
			t.Errorf("chord prefix %v offers no which-key hint", group.Sequence)
		}
		for _, hint := range hints {
			if hint.Label == "" {
				t.Errorf("chord prefix %v hints key %q without a label", group.Sequence, hint.Key)
			}
			path := append(slices.Clone(group.Sequence), hint.Key)
			if _, _, resolution := registry.InputBindingForSequence(
				operation.InputStrategySimplified,
				path,
				ctx,
			); resolution == operation.ChordNoMatch {
				t.Errorf("which-key row offers %v, which resolves to nothing", path)
			}
		}
	}
}
