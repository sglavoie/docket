package tui

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// contextActionTestDescriptor is a global, value-free operation so engine tests
// exercise sheet behavior instead of task target resolution.
func contextActionTestDescriptor(id operation.OperationID, label string) operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          operation.OperationCategoryScope,
		TargetSource:      operation.OperationTargetGlobal,
		TargetCardinality: operation.TargetCardinalityNone,
		Risk:              operation.OperationRiskMutation,
	}
}

func contextActionTestEngine(
	t *testing.T,
	descriptors []operation.OperationDescriptor,
	handlers map[operation.OperationID]operation.OperationHandler,
) (*operation.OperationRegistry, *operation.OperationExecutor) {
	t.Helper()
	registry := operation.NewOperationRegistry()
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register %s: %v", descriptor.ID, err)
		}
	}
	executor := operation.NewOperationExecutor(registry)
	for id, handler := range handlers {
		if err := executor.Register(operation.OperationBinding{
			OperationID: id,
			Handler:     handler,
		}); err != nil {
			t.Fatalf("bind %s: %v", id, err)
		}
	}
	return registry, executor
}

func contextActionTestContext() operation.CommandContext {
	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	return context
}

func TestContextActionSheetPreservesHostOrderAndDropsUnavailableOperations(t *testing.T) {
	blocked := contextActionTestDescriptor("test.blocked", "Blocked")
	blocked.Applicability = func(operation.CommandContext) bool { return false }
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.zeta", "Zeta"),
		contextActionTestDescriptor("test.alpha", "Alpha"),
		blocked,
	}, nil)

	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]operation.OperationID{"test.zeta", "test.alpha", "test.zeta", "test.blocked", "test.missing"},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostProjectHub},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	want := []operation.OperationID{"test.zeta", "test.alpha"}
	if got := sheet.OperationIDs(); !slices.Equal(got, want) {
		t.Fatalf("sheet operations = %v, want host order %v", got, want)
	}
	if got := sheet.Surface.Items()[0].ID; got != "project-hub-action:test.zeta" {
		t.Fatalf("item ID = %q, want the host-namespaced default prefix", got)
	}
}

func TestContextActionSheetFreezesHostContextAndValue(t *testing.T) {
	var invocation operation.OperationInvocation
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, map[operation.OperationID]operation.OperationHandler{
		"test.act": func(got operation.OperationInvocation) operation.OperationHandlerResult {
			invocation = got
			return operation.OperationHandlerResult{Status: "acted"}
		},
	})

	context := contextActionTestContext()
	context.ScopeProjectIDs = []string{"project-a"}
	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		context,
		[]operation.OperationID{"test.act"},
		listsurface.ContextActionSheetConfig{
			Host:     listsurface.ContextActionHostProjectHub,
			Value:    "frozen-row",
			HasValue: true,
		},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	// A host that keeps moving beneath the modal must not retarget the sheet.
	context.ScopeProjectIDs[0] = "project-b"

	_, dismissed, raw := sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("a completed action did not dismiss the sheet")
	}
	emitted, ok := raw.(listsurface.ContextActionSheetResultMsg)
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
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.fail", "Fail"),
		contextActionTestDescriptor("test.ok", "Ok"),
	}, map[operation.OperationID]operation.OperationHandler{
		"test.fail": func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Err: errors.New("handler refused")}
		},
		"test.ok": func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Status: "done"}
		},
	})

	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]operation.OperationID{"test.fail", "test.ok"},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}

	if !sheet.Surface.SetCursorToID("outliner-action:test.fail") {
		t.Fatal("failing action is missing from the sheet")
	}
	_, dismissed, raw := sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a failed action dismissed the sheet")
	}
	if emitted := raw.(listsurface.ContextActionSheetResultMsg); emitted.Result.Err == nil {
		t.Fatal("failed execution reported no error")
	}
	if sheet.Notice == "" {
		t.Fatal("sheet did not retain the failure reason")
	}

	if !sheet.Surface.SetCursorToID("outliner-action:test.ok") {
		t.Fatal("succeeding action is missing from the sheet")
	}
	if _, dismissed, _ = sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); !dismissed {
		t.Fatal("a completed action did not dismiss the sheet")
	}
	if sheet.Notice != "" {
		t.Fatalf("completed action kept a stale notice: %q", sheet.Notice)
	}
}

func TestContextActionSheetEscReturnsToHostWithoutExecuting(t *testing.T) {
	executed := false
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, map[operation.OperationID]operation.OperationHandler{
		"test.act": func(operation.OperationInvocation) operation.OperationHandlerResult {
			executed = true
			return operation.OperationHandlerResult{}
		},
	})

	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		[]operation.OperationID{"test.act"},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostProjectHub},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	_, dismissed, raw := sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dismissed {
		t.Fatal("Esc did not close the sheet")
	}
	if _, ok := raw.(listsurface.ContextActionSheetClosedMsg); !ok {
		t.Fatalf("Esc returned %T, want ContextActionSheetClosedMsg", raw)
	}
	if executed {
		t.Fatal("Esc executed the cursor action")
	}
}

func TestContextActionSheetSheetOnlyCompletionLeavesHostOpen(t *testing.T) {
	app := newUITestApp()
	if cmd := app.operations.ApplySheetResult(app, listsurface.ContextActionSheetResultMsg{}); cmd != nil {
		t.Fatalf("close-sheet completion asked the host to close: %T", cmd())
	}
}

func TestContextActionSheetConfirmationCancelReturnsToSheet(t *testing.T) {
	app := newUITestApp()
	executed := false
	descriptor := contextActionTestDescriptor("test.destroy", "Destroy")
	descriptor.Risk = operation.OperationRiskDestructive
	if err := app.operations.Registry.Register(descriptor); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := app.operations.Executor.Register(operation.OperationBinding{
		OperationID: descriptor.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			executed = true
			return operation.OperationHandlerResult{Status: "destroyed"}
		},
	}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	sheet, err := listsurface.NewContextActionSheetModel(
		app.operations.Registry,
		app.operations.Executor,
		contextActionTestContext(),
		[]operation.OperationID{descriptor.ID},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	app.operations.ActionSheet = sheet

	_, dismissed, raw := sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a pending confirmation dismissed the sheet")
	}
	if handled, cmd := app.handleOperationOverlayMessage(raw); !handled || cmd != nil {
		t.Fatalf("confirmation request = handled %v cmd %T", handled, cmd)
	}
	if !app.operations.Confirm.IsActive() {
		t.Fatal("destructive action did not prompt for confirmation")
	}
	if executed {
		t.Fatal("destructive handler ran before confirmation")
	}

	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || cmd != nil {
		t.Fatalf("confirmation Esc = handled %v cmd %T", handled, cmd)
	}
	if app.operations.Confirm.IsActive() {
		t.Fatal("Esc did not cancel the confirmation")
	}
	if executed {
		t.Fatal("canceled confirmation still executed the action")
	}
	if app.operations.ActionSheet != sheet {
		t.Fatal("canceled confirmation did not return to the same sheet")
	}

	// Re-confirming from the retained sheet runs the frozen request exactly once.
	_, _, raw = sheet.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if handled, _ := app.handleOperationOverlayMessage(raw); !handled {
		t.Fatal("second confirmation request was not handled")
	}
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'y', Text: "y"}); !handled {
		t.Fatal("confirmation y was not handled")
	}
	if !executed {
		t.Fatal("confirmed destructive action never executed")
	}
	if app.operations.ActionSheet != nil {
		t.Fatal("confirmed action did not close the sheet")
	}
}

func TestListActionStateIntersectionFeedsContextActionSheet(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.shared", "Shared"),
		contextActionTestDescriptor("test.only-first", "Only First"),
	}, nil)

	surface, err := listsurface.NewListSurfaceModel([]listsurface.ListItem{
		{
			ID:           "row:first",
			PrimaryText:  "First",
			OperationIDs: []operation.OperationID{"test.shared", "test.only-first"},
		},
		{
			ID:           "row:second",
			PrimaryText:  "Second",
			OperationIDs: []operation.OperationID{"test.shared"},
		},
	}, nil, listsurface.ListSurfaceConfig{SelectionMode: listsurface.ListSelectionMulti})
	if err != nil {
		t.Fatalf("NewListSurfaceModel: %v", err)
	}
	if selectErr := surface.SetSelectedIDs([]listsurface.ListItemID{"row:first", "row:second"}); selectErr != nil {
		t.Fatalf("SetSelectedIDs: %v", selectErr)
	}

	state := surface.ActionState()
	if !state.Enabled() {
		t.Fatalf("multi-row action state is disabled: %q", state.DisabledReason)
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		registry,
		executor,
		contextActionTestContext(),
		state.OperationIDs,
		listsurface.ContextActionSheetConfig{
			Host:     listsurface.ContextActionHostProjectHub,
			Targets:  len(state.ItemIDs),
			Bulk:     true,
			Value:    state.ItemIDs,
			HasValue: true,
		},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}
	if got := sheet.OperationIDs(); !slices.Equal(got, []operation.OperationID{"test.shared"}) {
		t.Fatalf("sheet operations = %v, want only the intersection", got)
	}
}

func TestContextActionSheetTaskAdapterKeepsStableItemIdentity(t *testing.T) {
	registry := operation.NewOperationRegistry()
	for _, descriptor := range taskActionTestDescriptors() {
		registerTaskActionTestDescriptor(t, registry, descriptor)
	}
	sheet, err := NewTaskActionSheetModel(
		registry,
		operation.NewOperationExecutor(registry),
		taskActionTestContext(),
		"Selected task",
	)
	if err != nil {
		t.Fatalf("NewTaskActionSheetModel: %v", err)
	}
	if sheet.Config.Host != listsurface.ContextActionHostOutliner ||
		sheet.Config.Completion != listsurface.ContextActionCompleteCloseSheet {
		t.Fatalf("task sheet lifecycle = %+v", sheet.Config)
	}
	if !sheet.Surface.SetCursorToID(listsurface.ListItemID("task-action:" + string(operation.OperationTaskDelete))) {
		t.Fatal("task sheet lost its stable task-action item IDs")
	}
}
