package tui

import (
	"slices"
	"strings"
	"testing"
)

// parityAuditApp builds the production catalog exactly as NewAppModel does, so
// these invariants audit registerOperationCatalog itself rather than a
// test-local subset of it.
func parityAuditApp(t *testing.T) *AppModel {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver.App()
}

func TestEveryRegisteredOperationHasOneCompatibleHandler(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	executor := app.operationExecutor

	for _, descriptor := range registry.All() {
		bindings := executor.bindings[descriptor.ID]
		if len(bindings) == 0 {
			t.Errorf("operation %s has a descriptor but no executable binding", descriptor.ID)
			continue
		}
		unconditional := 0
		for _, binding := range bindings {
			if len(binding.Views) == 0 && binding.Applicability == nil && len(binding.RequiredCapabilities) == 0 {
				unconditional++
			}
			switch descriptor.TargetCardinality {
			case TargetCardinalityNone:
				if binding.Handler == nil {
					t.Errorf("operation %s resolves no targets but binds a target handler", descriptor.ID)
				}
			case TargetCardinalityOneOrMore:
				// executeOperationBinding falls through to ErrOperationHandler when a
				// SingleHandler receives more than one target, so a bulk-capable
				// descriptor bound that way fails only once a user selects two rows.
				if binding.SingleHandler != nil {
					t.Errorf("operation %s accepts many targets but binds a single-target handler", descriptor.ID)
				}
			case TargetCardinalityOne:
			}
		}
		if unconditional > 1 {
			t.Errorf("operation %s has %d unconditional bindings, want at most 1", descriptor.ID, unconditional)
		}
	}
}

// delegatedDestructiveConfirmations are the destructive operations whose
// handler owns a canonical confirmation prompt of its own. Every other
// destructive operation must fall through to the executor gate; adding a new
// exemption here is a deliberate claim that the handler prompts before writing.
var delegatedDestructiveConfirmations = map[OperationID]string{
	operationTaskDelete:          "handleTaskDeleteRequested prompts before confirmTaskDelete writes",
	operationScopeArchiveProject: "showProjectArchiveConfirm prompts",
	operationScopeDeleteContext:  "requestDeleteSelected shows the Scope Hub confirm",
}

func TestEveryDestructiveOperationPromptsBeforeItWrites(t *testing.T) {
	app := parityAuditApp(t)
	executor := app.operationExecutor

	destructive := 0
	for _, descriptor := range app.GetOperationRegistry().All() {
		if descriptor.Risk != OperationRiskDestructive {
			continue
		}
		destructive++
		reason, exempt := delegatedDestructiveConfirmations[descriptor.ID]
		for _, binding := range executor.bindings[descriptor.ID] {
			if !binding.DelegateConfirmation {
				continue
			}
			if !exempt {
				t.Errorf(
					"destructive operation %s delegates confirmation without a documented prompt owner",
					descriptor.ID,
				)
			}
		}
		if exempt && reason == "" {
			t.Errorf("destructive operation %s is exempt without a reason", descriptor.ID)
		}
		// A value picker ahead of the gate would let the user pick a value before
		// being asked whether the destruction should happen at all.
		if descriptor.ValueProviderID != "" {
			t.Errorf(
				"destructive operation %s requests a value before the confirmation gate",
				descriptor.ID,
			)
		}
	}
	if destructive == 0 {
		t.Fatal("no destructive operations found; the invariant is not auditing anything")
	}
	for id := range delegatedDestructiveConfirmations {
		descriptor, ok := app.GetOperationRegistry().Descriptor(id)
		if !ok {
			t.Errorf("exempt operation %s is no longer registered", id)
			continue
		}
		if descriptor.Risk != OperationRiskDestructive {
			t.Errorf("exempt operation %s is no longer destructive", id)
		}
	}
}

func TestHarnessAndRetiredOperationsStayOutOfProductionSurfaces(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	ctx := app.operationCommandContext()

	strategyOperations := make(map[OperationID]bool)
	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for _, binding := range strategy.Bindings {
		strategyOperations[binding.OperationID] = true
	}

	harness := 0
	for _, descriptor := range registry.All() {
		if !descriptor.RequiredCapabilities.Has(CapabilityHarness) {
			continue
		}
		harness++
		if descriptor.Applicable(ctx) {
			t.Errorf("harness operation %s is applicable in a production context", descriptor.ID)
		}
		if strategyOperations[descriptor.ID] {
			t.Errorf("harness operation %s is bound to a production direct key", descriptor.ID)
		}
	}
	if harness == 0 {
		t.Fatal("no harness operation was registered, so the exclusion proves nothing")
	}

	// Restoring discovery must not restore the retired palette grammar or the
	// screens the simplified daily loop replaced.
	retired := []string{
		"palette", "prefix", "quick-capture", "bulk-capture", "quick-switch",
		"focus-board", "attr-filter", "pinned-tasks", "recent-tasks",
		"inbox-processor", "archive-all",
	}
	for _, descriptor := range registry.All() {
		id := strings.ToLower(string(descriptor.ID))
		label := strings.ToLower(descriptor.Label)
		for _, name := range retired {
			spelled := strings.ReplaceAll(name, "-", " ")
			if strings.Contains(id, name) || strings.Contains(label, spelled) {
				t.Errorf("retired operation %s (%q) returned to the registry", descriptor.ID, descriptor.Label)
			}
		}
	}
}

func TestDirectKeysHelpAndHandledKeysDeriveFromTheInputStrategy(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	// Task-scoped keys only resolve over a real target, so the audit runs in the
	// same focused context the user is in when they press them.
	ctx := taskActionParityContext(t, app)

	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	handled := registry.HandledKeysForInputStrategy(InputStrategySimplified)
	expectedKeys := make([]string, 0, len(handled))
	for _, binding := range strategy.Bindings {
		for _, path := range binding.paths() {
			for _, key := range path {
				if !slices.Contains(expectedKeys, key) {
					expectedKeys = append(expectedKeys, key)
				}
			}
			resolved, descriptor, resolution := registry.InputBindingForSequence(InputStrategySimplified, path, ctx)
			if resolution != ChordTerminal {
				t.Errorf("key path %v does not resolve to an operation in the Outliner context", path)
				continue
			}
			if resolved.OperationID != binding.OperationID || descriptor.ID != binding.OperationID {
				t.Errorf("key path %v resolved to %s, want %s", path, descriptor.ID, binding.OperationID)
			}
			// A chord's own prefixes must wait for the next key rather than run.
			for depth := 1; depth < len(path); depth++ {
				_, _, resolution := registry.InputBindingForSequence(InputStrategySimplified, path[:depth], ctx)
				if resolution != ChordPending {
					t.Errorf("chord prefix %v resolution = %v, want pending", path[:depth], resolution)
				}
			}
		}
	}
	if !slices.Equal(handled, expectedKeys) {
		t.Errorf("handled keys = %v, want the strategy keys %v", handled, expectedKeys)
	}

	help := registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx)
	applicable := registry.InputStrategyOperations(InputStrategySimplified, ctx)
	if len(help) != len(applicable) {
		t.Fatalf("help items = %d, want one per applicable binding (%d)", len(help), len(applicable))
	}
	for index, item := range help {
		descriptor := applicable[index]
		if item.Description != descriptor.Label {
			t.Errorf("help row %d describes %q, want the descriptor label %q", index, item.Description, descriptor.Label)
		}
		shortcut := registry.InputShortcutForOperation(InputStrategySimplified, descriptor.ID)
		if item.Key != shortcut {
			t.Errorf("help row %d shows key %q, want the strategy display key %q", index, item.Key, shortcut)
		}
	}
}

// TestChordBindingsStayPrefixFreeExecutableAndLabeled audits the chord half of
// the one grammar: no key path shadows another, presets match the value their
// handler asserts, every chord reaches a compatible handler, and every prefix
// can render a which-key row. It is quiet while the strategy declares no chord.
func TestChordBindingsStayPrefixFreeExecutableAndLabeled(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	executor := app.operationExecutor
	ctx := taskActionParityContext(t, app)

	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	paths := make([][]string, 0, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		paths = append(paths, binding.paths()...)
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
		if !descriptor.ValueProviderID.acceptsPresetValue(binding.PresetValue) {
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
		hints := registry.ChordHintsForPrefix(InputStrategySimplified, group.Sequence, ctx)
		if len(hints) == 0 {
			t.Errorf("chord prefix %v offers no which-key hint", group.Sequence)
		}
		for _, hint := range hints {
			if hint.Label == "" {
				t.Errorf("chord prefix %v hints key %q without a label", group.Sequence, hint.Key)
			}
			path := append(slices.Clone(group.Sequence), hint.Key)
			if _, _, resolution := registry.InputBindingForSequence(
				InputStrategySimplified,
				path,
				ctx,
			); resolution == ChordNoMatch {
				t.Errorf("which-key row offers %v, which resolves to nothing", path)
			}
		}
	}
}

func TestSpaceProjectionUsesTheDeclaredTaskManifest(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	executor := app.operationExecutor

	sheetContext := taskActionParityContext(t, app)
	descriptors := TaskActionSheetDescriptors(registry, sheetContext)
	if len(descriptors) == 0 {
		t.Fatal("the Space sheet offered no action for a live task")
	}
	seen := make(map[OperationID]bool)
	for _, descriptor := range descriptors {
		if seen[descriptor.ID] {
			t.Errorf("Space sheet lists %s twice", descriptor.ID)
		}
		seen[descriptor.ID] = true
		if len(executor.bindings[descriptor.ID]) == 0 {
			t.Errorf("Space sheet lists unexecutable operation %s", descriptor.ID)
		}
		entry, manifested := taskOperationManifestEntryFor(descriptor.ID)
		if !manifested {
			t.Errorf("Space sheet lists %s, which the task manifest does not own", descriptor.ID)
			continue
		}
		if entry.DirectOnly {
			t.Errorf("Space sheet repeats direct-only operation %s", descriptor.ID)
		}
		// Manifest actions reach users through the row they act on.
	}

	// The menu catalog excludes direct-only entries whose key the footer already
	// advertises.
	rowActions := taskOperationManifestIDs()
	for _, id := range rowActions {
		if _, ok := registry.Descriptor(id); !ok {
			t.Errorf("task manifest advertises unregistered operation %s", id)
		}
	}
	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for _, entry := range taskOperationManifest() {
		if !entry.DirectOnly {
			continue
		}
		if slices.Contains(rowActions, entry.ID) {
			t.Errorf("direct-only operation %s is still a row action", entry.ID)
		}
		// A removed projection has to leave a key behind, or the operation becomes
		// unreachable instead of faster.
		if displayKeysForOperation(strategy, entry.ID) == "" {
			t.Errorf("direct-only operation %s has no key in the input strategy", entry.ID)
		}
	}
}

// taskActionParityContext focuses the Outliner on the first live task so the
// sheet projection is evaluated against a real target instead of an empty one.
func taskActionParityContext(t *testing.T, app *AppModel) CommandContext {
	t.Helper()
	outliner := app.crossOutliner
	if outliner == nil {
		t.Fatal("fixture app has no Outliner")
	}
	for index, row := range outliner.rows {
		if !row.isTaskRow() {
			continue
		}
		outliner.cursor = index
		ctx := outliner.operationCommandContext()
		if currentTaskIdentity(ctx).Valid() {
			return ctx
		}
	}
	t.Fatal("fixture Outliner exposed no task row")
	return CommandContext{}
}
