package tui

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type operationTestMsg string

type operationTestTransaction struct {
	commits   int
	rollbacks int
	commitErr error
}

func (transaction *operationTestTransaction) Commit() error {
	transaction.commits++
	return transaction.commitErr
}

func (transaction *operationTestTransaction) Rollback() error {
	transaction.rollbacks++
	return nil
}

func TestOperationTargetResolverResolvesEveryTargetSourceDeterministically(t *testing.T) {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Entity = EntityContext{ProjectID: "project-b", TaskID: "task-current"}
	ctx.Selection.Tasks = []TaskIdentity{
		{ProjectID: "project-b", TaskID: "task-2"},
		{ProjectID: "project-a", TaskID: "task-1"},
		{ProjectID: "project-b", TaskID: "task-2"},
		{ProjectID: "", TaskID: "invalid"},
	}
	ctx.ScopeProjectIDs = []string{"project-b", "project-a", "project-b"}

	tests := []struct {
		name         string
		source       OperationTargetSource
		category     OperationCategory
		cardinality  TargetCardinality
		wantTasks    []TaskIdentity
		wantProjects []string
		wantCount    int
	}{
		{
			name: "current task", source: OperationTargetCurrentTask,
			category:     OperationCategoryTask,
			cardinality:  TargetCardinalityOne,
			wantTasks:    []TaskIdentity{{ProjectID: "project-b", TaskID: "task-current"}},
			wantProjects: []string{"project-b"},
			wantCount:    1,
		},
		{
			name: "selected tasks", source: OperationTargetSelectedTasks,
			category:    OperationCategoryTask,
			cardinality: TargetCardinalityOneOrMore,
			wantTasks: []TaskIdentity{
				{ProjectID: "project-a", TaskID: "task-1"},
				{ProjectID: "project-b", TaskID: "task-2"},
			},
			wantProjects: []string{"project-a", "project-b"},
			wantCount:    2,
		},
		{
			name: "current project", source: OperationTargetCurrentProject,
			category:     OperationCategoryProject,
			cardinality:  TargetCardinalityOne,
			wantProjects: []string{"project-b"},
			wantCount:    1,
		},
		{
			name: "scoped project set", source: OperationTargetScopedProjects,
			category:     OperationCategoryProject,
			cardinality:  TargetCardinalityOneOrMore,
			wantProjects: []string{"project-a", "project-b"},
			wantCount:    2,
		},
		{
			name: "global", source: OperationTargetGlobal,
			category:    OperationCategorySystem,
			cardinality: TargetCardinalityNone,
			wantCount:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := OperationDescriptor{
				ID:                OperationID("test." + test.name),
				Label:             test.name,
				Category:          test.category,
				TargetSource:      test.source,
				TargetCardinality: test.cardinality,
				Risk:              OperationRiskReadOnly,
			}
			targets, err := (OperationTargetResolver{}).Resolve(descriptor, ctx)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got := targets.Count(); got != test.wantCount {
				t.Fatalf("Count = %d, want %d", got, test.wantCount)
			}
			if got := targets.Tasks(); !slices.Equal(got, test.wantTasks) {
				t.Fatalf("Tasks = %v, want %v", got, test.wantTasks)
			}
			if got := targets.ProjectIDs(); !slices.Equal(got, test.wantProjects) {
				t.Fatalf("ProjectIDs = %v, want %v", got, test.wantProjects)
			}
		})
	}
}

func TestOperationTargetResolverSelectedTasksFallsBackToCurrentTask(t *testing.T) {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Entity = EntityContext{ProjectID: "project", TaskID: "current"}
	descriptor := OperationDescriptor{
		ID:                "task.bulk",
		Label:             "Bulk task",
		Category:          OperationCategoryTask,
		TargetSource:      OperationTargetSelectedTasks,
		TargetCardinality: TargetCardinalityOneOrMore,
		Risk:              OperationRiskMutation,
	}

	targets, err := (OperationTargetResolver{}).Resolve(descriptor, ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []TaskIdentity{{ProjectID: "project", TaskID: "current"}}
	if got := targets.Tasks(); !slices.Equal(got, want) {
		t.Fatalf("Tasks = %v, want %v", got, want)
	}
}

func TestOperationExecutorCentralizesValueConfirmationTransactionStatusAndUndo(t *testing.T) {
	registry := NewOperationRegistry()
	descriptor := OperationDescriptor{
		ID:                "task.destroy",
		Label:             "Destroy selected tasks",
		Category:          OperationCategoryTask,
		TargetSource:      OperationTargetSelectedTasks,
		TargetCardinality: TargetCardinalityOneOrMore,
		Risk:              OperationRiskDestructive,
		ValueProviderID:   ValueProviderState,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)

	providerCalls := 0
	if err := executor.RegisterValueProvider(ValueProviderState, func(OperationInvocation) OperationValueProviderResult {
		providerCalls++
		return OperationValueProviderResult{Value: "done", HasValue: true}
	}); err != nil {
		t.Fatalf("RegisterValueProvider: %v", err)
	}

	transaction := &operationTestTransaction{}
	transactionBegins := 0
	executor.SetTransactionFactory(func(invocation OperationInvocation) (OperationTransaction, error) {
		transactionBegins++
		if got := invocation.Targets.Count(); got != 2 {
			t.Fatalf("transaction target count = %d, want 2", got)
		}
		return transaction, nil
	})

	reportedEvents := 0
	executor.SetStatusReporter(func(OperationStatusEvent) {
		reportedEvents++
	})

	handlerCalls := 0
	undoCalls := 0
	redoCalls := 0
	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		BulkHandler: func(invocation OperationInvocation, targets []OperationTarget) OperationHandlerResult {
			handlerCalls++
			if len(targets) != 2 {
				t.Fatalf("bulk targets = %d, want 2", len(targets))
			}
			if !invocation.HasValue || invocation.Value != "done" {
				t.Fatalf("value = (%v, %v), want (done, true)", invocation.Value, invocation.HasValue)
			}
			return OperationHandlerResult{
				Message: operationTestMsg("executed"),
				Status:  "Destroyed 2 tasks",
				Undo: func() tea.Msg {
					undoCalls++
					return operationTestMsg("undone")
				},
				Redo: func() tea.Msg {
					redoCalls++
					return operationTestMsg("redone")
				},
			}
		},
	}); err != nil {
		t.Fatalf("Register binding: %v", err)
	}

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Selection.Tasks = []TaskIdentity{
		{ProjectID: "project-b", TaskID: "task-2"},
		{ProjectID: "project-a", TaskID: "task-1"},
	}
	first := executor.Execute(OperationExecutionRequest{
		ID:      descriptor.ID,
		Context: ctx,
	})
	if first.ConfirmationRequired == nil {
		t.Fatal("destructive execution should require confirmation")
	}
	if handlerCalls != 0 || transactionBegins != 0 {
		t.Fatalf("pre-confirmation calls = handler %d transaction %d, want zero", handlerCalls, transactionBegins)
	}

	second := executor.Execute(first.ConfirmationRequired.Request)
	if second.Err != nil {
		t.Fatalf("confirmed Execute: %v", second.Err)
	}
	if got, ok := second.Message.(operationTestMsg); !ok || got != "executed" {
		t.Fatalf("message = %#v, want executed", second.Message)
	}
	if handlerCalls != 1 || transactionBegins != 1 || transaction.commits != 1 || transaction.rollbacks != 0 {
		t.Fatalf(
			"execution counts = handler %d begin %d commit %d rollback %d",
			handlerCalls,
			transactionBegins,
			transaction.commits,
			transaction.rollbacks,
		)
	}
	if providerCalls != 2 {
		t.Fatalf("value provider calls = %d, want once per execution attempt", providerCalls)
	}

	undo := executor.Undo()
	if got := undo.AsMessage(); got != operationTestMsg("undone") || undoCalls != 1 {
		t.Fatalf("Undo = (%#v, %d calls), want undone once", got, undoCalls)
	}
	redo := executor.Redo()
	if got := redo.AsMessage(); got != operationTestMsg("redone") || redoCalls != 1 {
		t.Fatalf("Redo = (%#v, %d calls), want redone once", got, redoCalls)
	}

	if reportedEvents != 6 {
		t.Fatalf("reported events = %d, want 6", reportedEvents)
	}
}

// TestOperationExecutionResultStatusDeliveryPrecedence pins the single outcome a
// result hands to the shell. A status is real feedback, but it never duplicates
// a message, an error, or a pending step, and never repeats what an installed
// reporter already showed.
func TestOperationExecutionResultStatusDeliveryPrecedence(t *testing.T) {
	failure := errors.New("copy failed")
	confirmation := &OperationConfirmationRequestMsg{Label: "Delete note"}
	value := &OperationValueRequestMsg{Provider: ValueProviderNoteTitle}

	tests := []struct {
		name   string
		result OperationExecutionResult
		want   tea.Msg
	}{
		{
			name:   "status only",
			result: OperationExecutionResult{Status: "Copied title to clipboard"},
			want:   StatusMsg("Copied title to clipboard"),
		},
		{
			name: "message outranks status",
			result: OperationExecutionResult{
				Message: operationTestMsg("executed"),
				Status:  "Destroyed 2 tasks",
			},
			want: operationTestMsg("executed"),
		},
		{
			name:   "error outranks status",
			result: OperationExecutionResult{Status: "Copied title to clipboard", Err: failure},
			want:   ErrorMsg{Err: failure},
		},
		{
			name:   "confirmation outranks status",
			result: OperationExecutionResult{Status: "Deleted", ConfirmationRequired: confirmation},
			want:   *confirmation,
		},
		{
			name:   "pending value outranks status",
			result: OperationExecutionResult{Status: "Renamed", ValueRequired: value},
			want:   *value,
		},
		{
			name:   "reported status is not delivered twice",
			result: OperationExecutionResult{Status: "Copied title to clipboard", StatusReported: true},
			want:   nil,
		},
		{
			name:   "silent outcome stays silent",
			result: OperationExecutionResult{},
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
	registry := NewOperationRegistry()
	reporting := OperationDescriptor{
		ID: "note.copy-title", Label: "Copy note title", Category: OperationCategoryNote,
		TargetSource: OperationTargetGlobal, TargetCardinality: TargetCardinalityNone,
		Risk: OperationRiskReadOnly,
	}
	silent := OperationDescriptor{
		ID: "core.back", Label: "Back", Category: OperationCategorySystem,
		TargetSource: OperationTargetGlobal, TargetCardinality: TargetCardinalityNone,
		Risk: OperationRiskReadOnly,
	}
	for _, descriptor := range []OperationDescriptor{reporting, silent} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := NewOperationExecutor(registry)
	if err := executor.Register(OperationBinding{
		OperationID: reporting.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{Status: "Copied title to clipboard"}
		},
	}); err != nil {
		t.Fatalf("Register reporting binding: %v", err)
	}
	if err := executor.Register(OperationBinding{
		OperationID: silent.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{}
		},
	}); err != nil {
		t.Fatalf("Register silent binding: %v", err)
	}

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	copied := executor.Execute(OperationExecutionRequest{ID: reporting.ID, Context: ctx})
	if copied.Err != nil || copied.Message != nil {
		t.Fatalf("copy result = %#v, want a status-only outcome", copied)
	}
	if got := copied.AsMessage(); got != StatusMsg("Copied title to clipboard") {
		t.Fatalf("copy message = %#v, want the handler status", got)
	}

	quiet := executor.Execute(OperationExecutionRequest{ID: silent.ID, Context: ctx})
	if quiet.Status != "" || quiet.AsMessage() != nil {
		t.Fatalf("silent result = %#v, want no user-visible feedback", quiet)
	}

	var events []OperationStatusEvent
	executor.SetStatusReporter(func(event OperationStatusEvent) {
		events = append(events, event)
	})
	reported := executor.Execute(OperationExecutionRequest{ID: reporting.ID, Context: ctx})
	if !reported.StatusReported || reported.AsMessage() != nil {
		t.Fatalf("reported result = %#v, want the reporter to own delivery", reported)
	}
	if executor.Execute(OperationExecutionRequest{ID: silent.ID, Context: ctx}).AsMessage() != nil {
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
	registry := NewOperationRegistry()
	speaking := OperationDescriptor{
		ID: "task.done", Label: "Complete task", Category: OperationCategoryTask,
		TargetSource: OperationTargetGlobal, TargetCardinality: TargetCardinalityNone,
		Risk: OperationRiskReadOnly,
	}
	statusOnly := OperationDescriptor{
		ID: "note.copy-title", Label: "Copy note title", Category: OperationCategoryNote,
		TargetSource: OperationTargetGlobal, TargetCardinality: TargetCardinalityNone,
		Risk: OperationRiskReadOnly,
	}
	for _, descriptor := range []OperationDescriptor{speaking, statusOnly} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := NewOperationExecutor(registry)
	if err := executor.Register(OperationBinding{
		OperationID: speaking.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{
				Message: operationTestMsg("completed"),
				Status:  "Completed 1 task",
			}
		},
	}); err != nil {
		t.Fatalf("Register message binding: %v", err)
	}
	if err := executor.Register(OperationBinding{
		OperationID: statusOnly.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{Status: "Copied title to clipboard"}
		},
	}); err != nil {
		t.Fatalf("Register status binding: %v", err)
	}

	shown := 0
	executor.SetStatusReporter(func(event OperationStatusEvent) {
		if event.OwnsUserFeedback {
			shown++
		}
	})

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	both := executor.Execute(OperationExecutionRequest{ID: speaking.ID, Context: ctx})
	if got := both.AsMessage(); got != operationTestMsg("completed") {
		t.Fatalf("message seam = %#v, want the handler message", got)
	}
	if both.StatusReported {
		t.Fatal("a message-plus-status outcome claimed the reporter had shown its status")
	}
	if shown != 0 {
		t.Fatalf("reporter showed %d statuses, want the message to own delivery", shown)
	}

	copied := executor.Execute(OperationExecutionRequest{ID: statusOnly.ID, Context: ctx})
	if !copied.StatusReported || copied.AsMessage() != nil {
		t.Fatalf("status-only result = %#v, want the reporter to own delivery", copied)
	}
	if shown != 1 {
		t.Fatalf("reporter showed %d statuses, want one", shown)
	}
}

func TestOperationExecutorRollsBackFailedHandler(t *testing.T) {
	registry := NewOperationRegistry()
	descriptor := OperationDescriptor{
		ID:                "global.mutate",
		Label:             "Mutate",
		Category:          OperationCategorySystem,
		TargetSource:      OperationTargetGlobal,
		TargetCardinality: TargetCardinalityNone,
		Risk:              OperationRiskMutation,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)
	transaction := &operationTestTransaction{}
	executor.SetTransactionFactory(func(OperationInvocation) (OperationTransaction, error) {
		return transaction, nil
	})
	handlerErr := errors.New("mutation failed")
	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{Err: handlerErr}
		},
	}); err != nil {
		t.Fatalf("Register binding: %v", err)
	}

	result := executor.Execute(OperationExecutionRequest{
		ID:      descriptor.ID,
		Context: PresentationCommandContext(ViewUnknown),
	})
	if !errors.Is(result.Err, handlerErr) {
		t.Fatalf("Execute error = %v, want %v", result.Err, handlerErr)
	}
	if transaction.commits != 0 || transaction.rollbacks != 1 {
		t.Fatalf("transaction = %d commits %d rollbacks, want 0/1", transaction.commits, transaction.rollbacks)
	}
}

func TestOperationExecutorSupportsSingleAndBulkHandlers(t *testing.T) {
	registry := NewOperationRegistry()
	singleDescriptor := OperationDescriptor{
		ID: "task.single", Label: "Single", Category: OperationCategoryTask,
		TargetSource: OperationTargetCurrentTask, TargetCardinality: TargetCardinalityOne,
		Risk: OperationRiskReadOnly,
	}
	bulkDescriptor := OperationDescriptor{
		ID: "task.bulk", Label: "Bulk", Category: OperationCategoryTask,
		TargetSource: OperationTargetSelectedTasks, TargetCardinality: TargetCardinalityOneOrMore,
		Risk: OperationRiskReadOnly,
	}
	for _, descriptor := range []OperationDescriptor{singleDescriptor, bulkDescriptor} {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	executor := NewOperationExecutor(registry)
	if err := executor.Register(OperationBinding{
		OperationID: singleDescriptor.ID,
		SingleHandler: func(_ OperationInvocation, target OperationTarget) OperationHandlerResult {
			return OperationHandlerResult{Message: operationTestMsg(target.Task.TaskID)}
		},
	}); err != nil {
		t.Fatalf("Register single: %v", err)
	}
	if err := executor.Register(OperationBinding{
		OperationID: bulkDescriptor.ID,
		BulkHandler: func(_ OperationInvocation, targets []OperationTarget) OperationHandlerResult {
			return OperationHandlerResult{Message: operationTestMsg("2")}
		},
	}); err != nil {
		t.Fatalf("Register bulk: %v", err)
	}

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Entity = EntityContext{ProjectID: "project", TaskID: "current"}
	ctx.Selection.Tasks = []TaskIdentity{
		{ProjectID: "project", TaskID: "one"},
		{ProjectID: "project", TaskID: "two"},
	}
	if got := executor.Execute(OperationExecutionRequest{ID: singleDescriptor.ID, Context: ctx}).Message; got != operationTestMsg("current") {
		t.Fatalf("single message = %#v, want current", got)
	}
	if got := executor.Execute(OperationExecutionRequest{ID: bulkDescriptor.ID, Context: ctx}).Message; got != operationTestMsg("2") {
		t.Fatalf("bulk message = %#v, want 2", got)
	}
}
