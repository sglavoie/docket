package tui

import (
	"errors"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestOperationExecutionResultStatusDeliveryPrecedence(t *testing.T) {
	failure := errors.New("copy failed")
	confirmation := &operation.OperationConfirmationRequestMsg{Label: "Delete Note"}
	value := &operation.OperationValueRequestMsg{Provider: operation.ValueProviderNoteTitle}

	tests := []struct {
		name   string
		result operation.OperationExecutionResult
		want   tea.Msg
	}{
		{
			name:   "status only",
			result: operation.OperationExecutionResult{Status: "Copied title to clipboard"},
			want:   status.StatusMsg("Copied title to clipboard"),
		},
		{
			name: "message outranks status",
			result: operation.OperationExecutionResult{
				Message: operationTestMsg("executed"),
				Status:  "Destroyed 2 tasks",
			},
			want: operationTestMsg("executed"),
		},
		{
			name:   "error outranks status",
			result: operation.OperationExecutionResult{Status: "Copied title to clipboard", Err: failure},
			want:   status.ErrorMsg{Err: failure},
		},
		{
			name:   "confirmation outranks status",
			result: operation.OperationExecutionResult{Status: "Deleted", ConfirmationRequired: confirmation},
			want:   *confirmation,
		},
		{
			name:   "pending value outranks status",
			result: operation.OperationExecutionResult{Status: "Renamed", ValueRequired: value},
			want:   *value,
		},
		{
			name:   "reported status is not delivered twice",
			result: operation.OperationExecutionResult{Status: "Copied title to clipboard", StatusReported: true},
			want:   nil,
		},
		{
			name:   "silent outcome stays silent",
			result: operation.OperationExecutionResult{},
			want:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.result.AsMessage(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("AsMessage = %#v, want %#v", got, test.want)
			}
		})
	}
}

// TestOperationExecutorStatusOnlyResultReachesTheMessageSeam covers the executor
// half of the same contract: a handler that only reports a status keeps it, a
// handler that reports nothing produces no toast even though the ledger and the
// reporter still name the operation, and an installed reporter takes over
// delivery.
func TestOperationExecutorStatusOnlyResultReachesTheMessageSeam(t *testing.T) {
	registry := operation.NewOperationRegistry()
	reporting := operation.OperationDescriptor{
		ID: "note.copy-title", Label: "Copy Note Title", Category: operation.OperationCategoryNote,
		TargetSource: operation.OperationTargetGlobal, TargetCardinality: operation.TargetCardinalityNone,
		Risk: operation.OperationRiskReadOnly,
	}
	silent := operation.OperationDescriptor{
		ID: "core.back", Label: "Back", Category: operation.OperationCategorySystem,
		TargetSource: operation.OperationTargetGlobal, TargetCardinality: operation.TargetCardinalityNone,
		Risk: operation.OperationRiskReadOnly,
	}
	for _, descriptor := range []operation.OperationDescriptor{reporting, silent} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := operation.NewOperationExecutor(registry)
	if err := executor.Register(operation.OperationBinding{
		OperationID: reporting.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Status: "Copied title to clipboard"}
		},
	}); err != nil {
		t.Fatalf("Register reporting binding: %v", err)
	}
	if err := executor.Register(operation.OperationBinding{
		OperationID: silent.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{}
		},
	}); err != nil {
		t.Fatalf("Register silent binding: %v", err)
	}

	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	copied := executor.Execute(operation.OperationExecutionRequest{ID: reporting.ID, Context: ctx})
	if copied.Err != nil || copied.Message != nil {
		t.Fatalf("copy result = %#v, want a status-only outcome", copied)
	}
	if got := copied.AsMessage(); got != status.StatusMsg("Copied title to clipboard") {
		t.Fatalf("copy message = %#v, want the handler status", got)
	}

	quiet := executor.Execute(operation.OperationExecutionRequest{ID: silent.ID, Context: ctx})
	if quiet.Status != "" || quiet.AsMessage() != nil {
		t.Fatalf("silent result = %#v, want no user-visible feedback", quiet)
	}

	var events []operation.OperationStatusEvent
	executor.SetStatusReporter(func(event operation.OperationStatusEvent) {
		events = append(events, event)
	})
	reported := executor.Execute(operation.OperationExecutionRequest{ID: reporting.ID, Context: ctx})
	if !reported.StatusReported || reported.AsMessage() != nil {
		t.Fatalf("reported result = %#v, want the reporter to own delivery", reported)
	}
	if executor.Execute(operation.OperationExecutionRequest{ID: silent.ID, Context: ctx}).AsMessage() != nil {
		t.Fatal("a silent operation produced a message once a reporter was installed")
	}
	if len(events) != 4 {
		t.Fatalf("events = %d, want start and completion for each execution", len(events))
	}
}

// TestOperationExecutorMessageAndStatusWithInstalledReporterDeliverOnce pins the
// delivery owner when both channels could speak. An installed reporter shows a
// completed status only when it is the outcome's whole feedback, so a handler
// that returns a message plus a status still reaches the user exactly once.
func TestOperationExecutorMessageAndStatusWithInstalledReporterDeliverOnce(t *testing.T) {
	registry := operation.NewOperationRegistry()
	speaking := operation.OperationDescriptor{
		ID: "task.done", Label: "Complete Task", Category: operation.OperationCategoryTask,
		TargetSource: operation.OperationTargetGlobal, TargetCardinality: operation.TargetCardinalityNone,
		Risk: operation.OperationRiskReadOnly,
	}
	statusOnly := operation.OperationDescriptor{
		ID: "note.copy-title", Label: "Copy Note Title", Category: operation.OperationCategoryNote,
		TargetSource: operation.OperationTargetGlobal, TargetCardinality: operation.TargetCardinalityNone,
		Risk: operation.OperationRiskReadOnly,
	}
	for _, descriptor := range []operation.OperationDescriptor{speaking, statusOnly} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := operation.NewOperationExecutor(registry)
	if err := executor.Register(operation.OperationBinding{
		OperationID: speaking.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{
				Message: operationTestMsg("completed"),
				Status:  "Completed 1 task",
			}
		},
	}); err != nil {
		t.Fatalf("Register message binding: %v", err)
	}
	if err := executor.Register(operation.OperationBinding{
		OperationID: statusOnly.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Status: "Copied title to clipboard"}
		},
	}); err != nil {
		t.Fatalf("Register status binding: %v", err)
	}

	shown := 0
	executor.SetStatusReporter(func(event operation.OperationStatusEvent) {
		if event.OwnsUserFeedback {
			shown++
		}
	})

	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	both := executor.Execute(operation.OperationExecutionRequest{ID: speaking.ID, Context: ctx})
	if got := both.AsMessage(); got != operationTestMsg("completed") {
		t.Fatalf("message seam = %#v, want the handler message", got)
	}
	if both.StatusReported {
		t.Fatal("a message-plus-status outcome claimed the reporter had shown its status")
	}
	if shown != 0 {
		t.Fatalf("reporter showed %d statuses, want the message to own delivery", shown)
	}

	copied := executor.Execute(operation.OperationExecutionRequest{ID: statusOnly.ID, Context: ctx})
	if !copied.StatusReported || copied.AsMessage() != nil {
		t.Fatalf("status-only result = %#v, want the reporter to own delivery", copied)
	}
	if shown != 1 {
		t.Fatalf("reporter showed %d statuses, want one", shown)
	}
}

func TestOperationExecutorRollsBackFailedHandler(t *testing.T) {
	registry := operation.NewOperationRegistry()
	descriptor := operation.OperationDescriptor{
		ID:                "global.mutate",
		Label:             "Mutate",
		Category:          operation.OperationCategorySystem,
		TargetSource:      operation.OperationTargetGlobal,
		TargetCardinality: operation.TargetCardinalityNone,
		Risk:              operation.OperationRiskMutation,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := operation.NewOperationExecutor(registry)
	transaction := &operationTestTransaction{}
	executor.SetTransactionFactory(func(operation.OperationInvocation) (operation.OperationTransaction, error) {
		return transaction, nil
	})
	handlerErr := errors.New("mutation failed")
	if err := executor.Register(operation.OperationBinding{
		OperationID: descriptor.ID,
		Handler: func(operation.OperationInvocation) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Err: handlerErr}
		},
	}); err != nil {
		t.Fatalf("Register binding: %v", err)
	}

	result := executor.Execute(operation.OperationExecutionRequest{
		ID:      descriptor.ID,
		Context: route.PresentationCommandContext(presentation.ViewUnknown),
	})
	if !errors.Is(result.Err, handlerErr) {
		t.Fatalf("Execute error = %v, want %v", result.Err, handlerErr)
	}
	if transaction.commits != 0 || transaction.rollbacks != 1 {
		t.Fatalf("transaction = %d commits %d rollbacks, want 0/1", transaction.commits, transaction.rollbacks)
	}
}

func TestOperationExecutorSupportsSingleAndBulkHandlers(t *testing.T) {
	registry := operation.NewOperationRegistry()
	singleDescriptor := operation.OperationDescriptor{
		ID: "task.single", Label: "Single", Category: operation.OperationCategoryTask,
		TargetSource: operation.OperationTargetCurrentTask, TargetCardinality: operation.TargetCardinalityOne,
		Risk: operation.OperationRiskReadOnly,
	}
	bulkDescriptor := operation.OperationDescriptor{
		ID: "task.bulk", Label: "Bulk", Category: operation.OperationCategoryTask,
		TargetSource: operation.OperationTargetSelectedTasks, TargetCardinality: operation.TargetCardinalityOneOrMore,
		Risk: operation.OperationRiskReadOnly,
	}
	for _, descriptor := range []operation.OperationDescriptor{singleDescriptor, bulkDescriptor} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := operation.NewOperationExecutor(registry)
	if err := executor.Register(operation.OperationBinding{
		OperationID: singleDescriptor.ID,
		SingleHandler: func(_ operation.OperationInvocation, target operation.OperationTarget) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Message: operationTestMsg(target.Task.TaskID)}
		},
	}); err != nil {
		t.Fatalf("Register single: %v", err)
	}
	if err := executor.Register(operation.OperationBinding{
		OperationID: bulkDescriptor.ID,
		BulkHandler: func(_ operation.OperationInvocation, targets []operation.OperationTarget) operation.OperationHandlerResult {
			return operation.OperationHandlerResult{Message: operationTestMsg("2")}
		},
	}); err != nil {
		t.Fatalf("Register bulk: %v", err)
	}

	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity = operation.EntityContext{ProjectID: "project", TaskID: "current"}
	ctx.Selection.Tasks = []taskref.TaskIdentity{
		{ProjectID: "project", TaskID: "one"},
		{ProjectID: "project", TaskID: "two"},
	}
	if got := executor.Execute(operation.OperationExecutionRequest{ID: singleDescriptor.ID, Context: ctx}).Message; got != operationTestMsg("current") {
		t.Fatalf("single message = %#v, want current", got)
	}
	if got := executor.Execute(operation.OperationExecutionRequest{ID: bulkDescriptor.ID, Context: ctx}).Message; got != operationTestMsg("2") {
		t.Fatalf("bulk message = %#v, want 2", got)
	}
}
