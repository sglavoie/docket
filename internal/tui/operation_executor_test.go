package tui

import (
	"errors"
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
		cardinality  TargetCardinality
		wantTasks    []TaskIdentity
		wantProjects []string
		wantCount    int
	}{
		{
			name: "current task", source: OperationTargetCurrentTask,
			cardinality:  TargetCardinalityOne,
			wantTasks:    []TaskIdentity{{ProjectID: "project-b", TaskID: "task-current"}},
			wantProjects: []string{"project-b"},
			wantCount:    1,
		},
		{
			name: "selected tasks", source: OperationTargetSelectedTasks,
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
			cardinality:  TargetCardinalityOne,
			wantProjects: []string{"project-b"},
			wantCount:    1,
		},
		{
			name: "scoped project set", source: OperationTargetScopedProjects,
			cardinality:  TargetCardinalityOneOrMore,
			wantProjects: []string{"project-a", "project-b"},
			wantCount:    2,
		},
		{
			name: "global", source: OperationTargetGlobal,
			cardinality: TargetCardinalityNone,
			wantCount:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := OperationDescriptor{
				ID:                OperationID("test." + test.name),
				Label:             test.name,
				Category:          OperationCategoryTask,
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

	var phases []OperationStatusPhase
	executor.SetStatusReporter(func(event OperationStatusEvent) {
		phases = append(phases, event.Phase)
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
		Source:  OperationSourceFinder,
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

	wantPhases := []OperationStatusPhase{
		OperationStatusStarted,
		OperationStatusConfirmationRequired,
		OperationStatusStarted,
		OperationStatusCompleted,
		OperationStatusUndone,
		OperationStatusRedone,
	}
	if !slices.Equal(phases, wantPhases) {
		t.Fatalf("status phases = %v, want %v", phases, wantPhases)
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
		Context: PresentationCommandContext(ViewDashboard),
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
