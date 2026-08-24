package tui

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// contextActionTestDescriptor is a global, value-free operation so engine tests
// exercise sheet behavior instead of task target resolution.
func contextActionTestDescriptor(id OperationID, label string) OperationDescriptor {
	return OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          OperationCategoryScope,
		TargetSource:      OperationTargetGlobal,
		TargetCardinality: TargetCardinalityNone,
		Risk:              OperationRiskMutation,
	}
}

func contextActionTestEngine(
	t *testing.T,
	descriptors []OperationDescriptor,
	handlers map[OperationID]OperationHandler,
) (*OperationRegistry, *OperationExecutor) {
	t.Helper()
	registry := NewOperationRegistry()
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register %s: %v", descriptor.ID, err)
		}
	}
	executor := NewOperationExecutor(registry)
	for id, handler := range handlers {
		if err := executor.Register(OperationBinding{
			OperationID: id,
			Handler:     handler,
		}); err != nil {
			t.Fatalf("bind %s: %v", id, err)
		}
	}
	return registry, executor
}

func contextActionTestContext() CommandContext {
	context := PresentationCommandContext(ViewCrossProjectOutliner)
	context.Capabilities.Add(CapabilitySimplifiedInput)
	return context
}

func TestContextActionSheetPreservesHostOrderAndDropsUnavailableOperations(t *testing.T) {
	blocked := contextActionTestDescriptor("test.blocked", "Blocked")
	blocked.Applicability = func(CommandContext) bool { return false }
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.zeta", "Zeta"),
		contextActionTestDescriptor("test.alpha", "Alpha"),
		blocked,
	}, nil)

	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]OperationID{"test.zeta", "test.alpha", "test.zeta", "test.blocked", "test.missing"},
		ContextActionSheetConfig{Host: ContextActionHostProjectHub},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	want := []OperationID{"test.zeta", "test.alpha"}
	if got := sheet.operationIDs(); !slices.Equal(got, want) {
		t.Fatalf("sheet operations = %v, want host order %v", got, want)
	}
	if got := sheet.surface.Items()[0].ID; got != "project-hub-action:test.zeta" {
		t.Fatalf("item ID = %q, want the host-namespaced default prefix", got)
	}
}

func TestContextActionSheetFreezesHostContextAndValue(t *testing.T) {
	var invocation OperationInvocation
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, map[OperationID]OperationHandler{
		"test.act": func(got OperationInvocation) OperationHandlerResult {
			invocation = got
			return OperationHandlerResult{Status: "acted"}
		},
	})

	context := contextActionTestContext()
	context.ScopeProjectIDs = []string{"project-a"}
	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		context,
		[]OperationID{"test.act"},
		ContextActionSheetConfig{
			Host:     ContextActionHostProjectHub,
			Value:    "frozen-row",
			HasValue: true,
		},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	// A host that keeps moving beneath the modal must not retarget the sheet.
	context.ScopeProjectIDs[0] = "project-b"

	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("a completed action did not dismiss the sheet")
	}
	emitted, ok := raw.(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("Enter returned %T, want ContextActionSheetResultMsg", raw)
	}
	if got := invocation.Context.ScopeProjectIDs; !slices.Equal(got, []string{"project-a"}) {
		t.Fatalf("frozen scope = %v, want the context captured at open time", got)
	}
	if invocation.Value != "frozen-row" || !invocation.HasValue {
		t.Fatalf("frozen host value = %#v (has %v)", invocation.Value, invocation.HasValue)
	}
	if emitted.Result.Status != "acted" {
		t.Fatalf("result status = %q, want the handler status", emitted.Result.Status)
	}
}

func TestContextActionSheetRetainsFailureAndClosesOnlyOnCompletion(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.fail", "Fail"),
		contextActionTestDescriptor("test.ok", "Ok"),
	}, map[OperationID]OperationHandler{
		"test.fail": func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{Err: errors.New("handler refused")}
		},
		"test.ok": func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{Status: "done"}
		},
	})

	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]OperationID{"test.fail", "test.ok"},
		ContextActionSheetConfig{Host: ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}

	if !sheet.surface.SetCursorToID("outliner-action:test.fail") {
		t.Fatal("failing action is missing from the sheet")
	}
	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a failed action dismissed the sheet")
	}
	if emitted := raw.(ContextActionSheetResultMsg); emitted.Result.Err == nil {
		t.Fatal("failed execution reported no error")
	}
	if sheet.notice == "" {
		t.Fatal("sheet did not retain the failure reason")
	}

	if !sheet.surface.SetCursorToID("outliner-action:test.ok") {
		t.Fatal("succeeding action is missing from the sheet")
	}
	if _, dismissed, _ = sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); !dismissed {
		t.Fatal("a completed action did not dismiss the sheet")
	}
	if sheet.notice != "" {
		t.Fatalf("completed action kept a stale notice: %q", sheet.notice)
	}
}

func TestContextActionSheetEscReturnsToHostWithoutExecuting(t *testing.T) {
	executed := false
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, map[OperationID]OperationHandler{
		"test.act": func(OperationInvocation) OperationHandlerResult {
			executed = true
			return OperationHandlerResult{}
		},
	})

	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]OperationID{"test.act"},
		ContextActionSheetConfig{Host: ContextActionHostProjectHub},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dismissed {
		t.Fatal("Esc did not close the sheet")
	}
	if _, ok := raw.(ContextActionSheetClosedMsg); !ok {
		t.Fatalf("Esc returned %T, want ContextActionSheetClosedMsg", raw)
	}
	if executed {
		t.Fatal("Esc executed the cursor action")
	}
}

func TestContextActionSheetSheetOnlyCompletionLeavesHostOpen(t *testing.T) {
	app := newUITestApp()
	if cmd := app.applyContextActionSheetResult(ContextActionSheetResultMsg{}); cmd != nil {
		t.Fatalf("close-sheet completion asked the host to close: %T", cmd())
	}
}

func TestContextActionSheetConfirmationCancelReturnsToSheet(t *testing.T) {
	app := newUITestApp()
	executed := false
	descriptor := contextActionTestDescriptor("test.destroy", "Destroy")
	descriptor.Risk = OperationRiskDestructive
	if err := app.operationRegistry.Register(descriptor); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := app.operationExecutor.Register(OperationBinding{
		OperationID: descriptor.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			executed = true
			return OperationHandlerResult{Status: "destroyed"}
		},
	}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	sheet, err := NewContextActionSheetModel(
		app.operationRegistry,
		app.operationExecutor,
		contextActionTestContext(),
		[]OperationID{descriptor.ID},
		ContextActionSheetConfig{Host: ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	app.contextActionSheet = sheet

	_, dismissed, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a pending confirmation dismissed the sheet")
	}
	if handled, cmd := app.handleOperationOverlayMessage(raw); !handled || cmd != nil {
		t.Fatalf("confirmation request = handled %v cmd %T", handled, cmd)
	}
	if !app.operationConfirm.IsActive() {
		t.Fatal("destructive action did not prompt for confirmation")
	}
	if executed {
		t.Fatal("destructive handler ran before confirmation")
	}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || cmd != nil {
		t.Fatalf("confirmation Esc = handled %v cmd %T", handled, cmd)
	}
	if app.operationConfirm.IsActive() {
		t.Fatal("Esc did not cancel the confirmation")
	}
	if executed {
		t.Fatal("canceled confirmation still executed the action")
	}
	if app.contextActionSheet != sheet {
		t.Fatal("canceled confirmation did not return to the same sheet")
	}

	// Re-confirming from the retained sheet runs the frozen request exactly once.
	_, _, raw = sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if handled, _ := app.handleOperationOverlayMessage(raw); !handled {
		t.Fatal("second confirmation request was not handled")
	}
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'y', Text: "y"}); !handled {
		t.Fatal("confirmation y was not handled")
	}
	if !executed {
		t.Fatal("confirmed destructive action never executed")
	}
	if app.contextActionSheet != nil {
		t.Fatal("confirmed action did not close the sheet")
	}
}

func TestListActionStateIntersectionFeedsContextActionSheet(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.shared", "Shared"),
		contextActionTestDescriptor("test.only-first", "Only First"),
	}, nil)

	surface, err := NewListSurfaceModel([]ListItem{
		{
			ID:           "row:first",
			PrimaryText:  "First",
			OperationIDs: []OperationID{"test.shared", "test.only-first"},
		},
		{
			ID:           "row:second",
			PrimaryText:  "Second",
			OperationIDs: []OperationID{"test.shared"},
		},
	}, nil, ListSurfaceConfig{SelectionMode: ListSelectionMulti})
	if err != nil {
		t.Fatalf("NewListSurfaceModel: %v", err)
	}
	if selectErr := surface.SetSelectedIDs([]ListItemID{"row:first", "row:second"}); selectErr != nil {
		t.Fatalf("SetSelectedIDs: %v", selectErr)
	}

	state := surface.ActionState()
	if !state.Enabled() {
		t.Fatalf("multi-row action state is disabled: %q", state.DisabledReason)
	}
	sheet, err := NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		state.OperationIDs,
		ContextActionSheetConfig{
			Host:     ContextActionHostProjectHub,
			Targets:  len(state.ItemIDs),
			Bulk:     true,
			Value:    state.ItemIDs,
			HasValue: true,
		},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	if got := sheet.operationIDs(); !slices.Equal(got, []OperationID{"test.shared"}) {
		t.Fatalf("sheet operations = %v, want only the intersection", got)
	}
}

func TestContextActionSheetTaskAdapterKeepsStableItemIdentity(t *testing.T) {
	registry := NewOperationRegistry()
	for _, descriptor := range taskActionTestDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		NewOperationExecutor(registry),
		taskActionTestContext(),
		"Selected task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	if sheet.config.Host != ContextActionHostOutliner ||
		sheet.config.Completion != ContextActionCompleteCloseSheet {
		t.Fatalf("task sheet lifecycle = %+v", sheet.config)
	}
	if !sheet.surface.SetCursorToID(ListItemID("task-action:" + string(operationTaskDelete))) {
		t.Fatal("task sheet lost its stable task-action item IDs")
	}
}
