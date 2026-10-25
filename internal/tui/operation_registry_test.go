package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

func operationTestDescriptor(id, label string) operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                operation.OperationID(id),
		Label:             label,
		Category:          operation.OperationCategoryTask,
		TargetCardinality: operation.TargetCardinalityOne,
		Risk:              operation.OperationRiskMutation,
	}
}

func TestOperationRegistryRejectsDuplicateIDsAndUnconditionalShortcutCollisions(t *testing.T) {
	registry := operation.NewOperationRegistry()
	first := operationTestDescriptor("task.edit", "Edit task")
	first.DirectKey = "e"
	if err := registry.Register(first); err != nil {
		t.Fatalf("Register first: %v", err)
	}

	duplicate := operationTestDescriptor("task.edit", "Rename task")
	if err := registry.Register(duplicate); err == nil || !strings.Contains(err.Error(), "operation ID") {
		t.Fatalf("Register duplicate ID error = %v, want operation ID error", err)
	}
	if got, _ := registry.Descriptor(first.ID); got.Label != first.Label {
		t.Fatalf("duplicate registration replaced label with %q", got.Label)
	}

	collision := operationTestDescriptor("task.expand-editor", "Expand editor")
	collision.DirectKey = "e"
	if err := registry.Register(collision); err == nil || !strings.Contains(err.Error(), "shortcut") {
		t.Fatalf("Register shortcut collision error = %v, want shortcut error", err)
	}

	projectOnly := operationTestDescriptor("project.edit", "Edit Project")
	projectOnly.DirectKey = "e"
	projectOnly.RequiredCapabilities = operation.NewCapabilitySet(operation.CapabilityProjectScope)
	taskOnly := operationTestDescriptor("task.edit-detail", "Edit task detail")
	taskOnly.DirectKey = "e"
	taskOnly.RequiredCapabilities = operation.NewCapabilitySet(operation.CapabilityTaskScope)
	scoped := operation.NewOperationRegistry()
	if err := scoped.Register(projectOnly); err != nil {
		t.Fatalf("Register project-only: %v", err)
	}
	if err := scoped.Register(taskOnly); err != nil {
		t.Fatalf("mutually exclusive scope shortcuts should coexist: %v", err)
	}
}

func TestOperationRegistryDetectsPredicateCollisionInActiveContext(t *testing.T) {
	registry := operation.NewOperationRegistry()
	ready := operationTestDescriptor("task.ready", "Ready task action")
	ready.DirectKey = "x"
	ready.Applicability = func(ctx operation.CommandContext) bool {
		return ctx.Entity.TaskID == "ready" || ctx.Entity.TaskID == "both"
	}
	inbox := operationTestDescriptor("task.inbox", "Inbox task action")
	inbox.DirectKey = "x"
	inbox.Applicability = func(ctx operation.CommandContext) bool {
		return ctx.Entity.TaskID == "inbox" || ctx.Entity.TaskID == "both"
	}
	if err := registry.Register(ready); err != nil {
		t.Fatalf("Register ready: %v", err)
	}
	if err := registry.Register(inbox); err != nil {
		t.Fatalf("Register inbox: %v", err)
	}

	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity.TaskID = "ready"
	applicable, err := registry.ForContext(ctx)
	if err != nil || len(applicable) != 1 || applicable[0].ID != ready.ID {
		t.Fatalf("ready context = (%v, %v), want only %s", applicable, err, ready.ID)
	}

	ctx.Entity.TaskID = "both"
	if err := registry.ValidateContext(ctx); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("both context error = %v, want active collision", err)
	}
}

func TestOperationRegistryOrderingAndCopiesAreDeterministic(t *testing.T) {
	registry := operation.NewOperationRegistry()
	descriptors := []operation.OperationDescriptor{
		{
			ID: "navigation.back", Label: "Back", Category: operation.OperationCategoryNavigation,
			TargetCardinality: operation.TargetCardinalityNone, Risk: operation.OperationRiskReadOnly,
		},
		{
			ID: "task.start", Label: "Start", Category: operation.OperationCategoryTask,
			Aliases:           []string{"begin", "Begin", " start "},
			TargetCardinality: operation.TargetCardinalityOne, Risk: operation.OperationRiskMutation,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityTaskScope),
		},
		{
			ID: "task.edit", Label: "Edit", Category: operation.OperationCategoryTask,
			TargetCardinality: operation.TargetCardinalityOne, Risk: operation.OperationRiskMutation,
		},
	}
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}

	first := registry.All()
	second := registry.All()
	gotIDs := []operation.OperationID{first[0].ID, first[1].ID, first[2].ID}
	wantIDs := []operation.OperationID{"task.edit", "task.start", "navigation.back"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("ordered IDs = %v, want %v", gotIDs, wantIDs)
	}
	if !slices.Equal(first[1].Aliases, []string{"begin"}) {
		t.Fatalf("normalized aliases = %v, want [begin]", first[1].Aliases)
	}
	first[1].Aliases[0] = "mutated"
	first[1].RequiredCapabilities.Add(operation.CapabilityAppScope)
	if second[1].Aliases[0] != "begin" || second[1].RequiredCapabilities.Has(operation.CapabilityAppScope) {
		t.Fatal("All returned aliases or capabilities backed by mutable registry storage")
	}
}
