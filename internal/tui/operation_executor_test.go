package tui

import (
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
