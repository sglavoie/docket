package tui

import (
	"slices"
	"strings"
	"testing"
)

func operationTestDescriptor(id, label string) OperationDescriptor {
	return OperationDescriptor{
		ID:                OperationID(id),
		Label:             label,
		Category:          OperationCategoryTask,
		TargetCardinality: TargetCardinalityOne,
		Risk:              OperationRiskMutation,
	}
}

func TestOperationRegistryRejectsDuplicateIDsAndUnconditionalShortcutCollisions(t *testing.T) {
	registry := NewOperationRegistry()
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

	projectOnly := operationTestDescriptor("project.edit", "Edit project")
	projectOnly.DirectKey = "e"
	projectOnly.RequiredCapabilities = NewCapabilitySet(CapabilityProjectScope)
	taskOnly := operationTestDescriptor("task.edit-detail", "Edit task detail")
	taskOnly.DirectKey = "e"
	taskOnly.RequiredCapabilities = NewCapabilitySet(CapabilityTaskScope)
	scoped := NewOperationRegistry()
	if err := scoped.Register(projectOnly); err != nil {
		t.Fatalf("Register project-only: %v", err)
	}
	if err := scoped.Register(taskOnly); err != nil {
		t.Fatalf("mutually exclusive scope shortcuts should coexist: %v", err)
	}
}

func TestOperationRegistryDetectsPredicateCollisionInActiveContext(t *testing.T) {
	registry := NewOperationRegistry()
	ready := operationTestDescriptor("task.ready", "Ready task action")
	ready.DirectKey = "x"
	ready.Applicability = func(ctx CommandContext) bool {
		return ctx.Entity.State == "ready" || ctx.Entity.State == "both"
	}
	inbox := operationTestDescriptor("task.inbox", "Inbox task action")
	inbox.DirectKey = "x"
	inbox.Applicability = func(ctx CommandContext) bool {
		return ctx.Entity.State == "inbox" || ctx.Entity.State == "both"
	}
	if err := registry.Register(ready); err != nil {
		t.Fatalf("Register ready: %v", err)
	}
	if err := registry.Register(inbox); err != nil {
		t.Fatalf("Register inbox: %v", err)
	}

	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Entity.State = "ready"
	applicable, err := registry.ForContext(ctx)
	if err != nil || len(applicable) != 1 || applicable[0].ID != ready.ID {
		t.Fatalf("ready context = (%v, %v), want only %s", applicable, err, ready.ID)
	}

	ctx.Entity.State = "both"
	if err := registry.ValidateContext(ctx); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("both context error = %v, want active collision", err)
	}
}

func TestOperationRegistryOrderingAndCopiesAreDeterministic(t *testing.T) {
	registry := NewOperationRegistry()
	descriptors := []OperationDescriptor{
		{
			ID: "navigation.back", Label: "Back", Category: OperationCategoryNavigation,
			TargetCardinality: TargetCardinalityNone, Risk: OperationRiskReadOnly,
		},
		{
			ID: "task.start", Label: "Start", Category: OperationCategoryTask,
			Aliases:           []string{"begin", "Begin", " start "},
			TargetCardinality: TargetCardinalityOne, Risk: OperationRiskMutation,
			RequiredCapabilities: NewCapabilitySet(CapabilityTaskScope),
		},
		{
			ID: "task.edit", Label: "Edit", Category: OperationCategoryTask,
			TargetCardinality: TargetCardinalityOne, Risk: OperationRiskMutation,
		},
	}
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}

	first := registry.All()
	second := registry.All()
	gotIDs := []OperationID{first[0].ID, first[1].ID, first[2].ID}
	wantIDs := []OperationID{"task.edit", "task.start", "navigation.back"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("ordered IDs = %v, want %v", gotIDs, wantIDs)
	}
	if !slices.Equal(first[1].Aliases, []string{"begin"}) {
		t.Fatalf("normalized aliases = %v, want [begin]", first[1].Aliases)
	}
	first[1].Aliases[0] = "mutated"
	first[1].RequiredCapabilities.Add(CapabilityAppScope)
	if second[1].Aliases[0] != "begin" || second[1].RequiredCapabilities.Has(CapabilityAppScope) {
		t.Fatal("All returned aliases or capabilities backed by mutable registry storage")
	}
}
