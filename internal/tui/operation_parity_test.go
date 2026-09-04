package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
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
		bindings := executor.Bindings(descriptor.ID)
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
			case operation.TargetCardinalityNone:
				if binding.Handler == nil {
					t.Errorf("operation %s resolves no targets but binds a target handler", descriptor.ID)
				}
			case operation.TargetCardinalityOneOrMore:
				// executeOperationBinding falls through to ErrOperationHandler when a
				// SingleHandler receives more than one target, so a bulk-capable
				// descriptor bound that way fails only once a user selects two rows.
				if binding.SingleHandler != nil {
					t.Errorf("operation %s accepts many targets but binds a single-target handler", descriptor.ID)
				}
			case operation.TargetCardinalityOne:
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
var delegatedDestructiveConfirmations = map[operation.OperationID]string{
	operationTaskDelete:          "handleTaskDeleteRequested prompts before confirmTaskDelete writes",
	operationScopeArchiveProject: "showProjectArchiveConfirm prompts",
	operationProjectDelete:       "showProjectDeleteConfirm prompts with the cascade note before commitProjectDelete writes",
	operationNoteDelete:          "deleteCurrentNote prompts on the Notes manager's own confirm dialog before performDelete writes",
	operationAttachmentDelete:    "confirmDeleteSelectedAttachment prompts on the panel's own confirm dialog before executeDelete writes",
	operationAttachmentUnlink:    "confirmUnlinkSelectedAttachment prompts on the panel's own confirm dialog before executeKeep writes",
	operationCommentDelete:       "confirmDeleteSelectedComment prompts on the panel's own confirm dialog before executeDelete writes",
}

func TestEveryDestructiveOperationPromptsBeforeItWrites(t *testing.T) {
	app := parityAuditApp(t)
	executor := app.operationExecutor

	destructive := 0
	for _, descriptor := range app.GetOperationRegistry().All() {
		if descriptor.Risk != operation.OperationRiskDestructive {
			continue
		}
		destructive++
		reason, exempt := delegatedDestructiveConfirmations[descriptor.ID]
		for _, binding := range executor.Bindings(descriptor.ID) {
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
		if descriptor.Risk != operation.OperationRiskDestructive {
			t.Errorf("exempt operation %s is no longer destructive", id)
		}
	}
}

func TestHarnessAndRetiredOperationsStayOutOfProductionSurfaces(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	ctx := app.operationCommandContext()

	strategyOperations := make(map[operation.OperationID]bool)
	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
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

// TestChordBindingsStayPrefixFreeExecutableAndLabeled audits the chord half of
// the one grammar: no key path shadows another, presets match the value their
// handler asserts, every chord reaches a compatible handler, and every prefix
// can render a which-key row. It is quiet while the strategy declares no chord.

// taskActionParityContext focuses the Outliner on the first live task so the
// sheet projection is evaluated against a real target instead of an empty one.
func taskActionParityContext(t *testing.T, app *AppModel) operation.CommandContext {
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
		if operation.CurrentTaskIdentity(ctx).Valid() {
			return ctx
		}
	}
	t.Fatal("fixture Outliner exposed no task row")
	return operation.CommandContext{}
}
