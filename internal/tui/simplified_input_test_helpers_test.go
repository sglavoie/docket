package tui

// The simplified-strategy registry and the command context it resolves against,
// built without a fully initialized AppModel. They are one unit: the context
// names the capability the registry's bindings are gated on, so a test that
// takes one and not the other resolves nothing.

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
)

func registeredSimplifiedOperationRegistry(t *testing.T) *operation.OperationRegistry {
	t.Helper()
	registry := operation.NewOperationRegistry()
	descriptors := append(operation.CoreInputOperationDescriptors(), simplifiedTaskActionDescriptors()...)
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("Register(%s): %v", descriptor.ID, err)
		}
	}
	if err := registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	return registry
}

func guidanceHintsText(hints []chrome.GuidanceHint) string {
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, chrome.FooterHelpSeparator)
}

// simplifiedTaskActionDescriptors supplies the task-scoped and filter
// descriptors the simplified strategy binds directly, for tests that assemble a
// registry without a fully initialized AppModel.
func simplifiedTaskActionDescriptors() []operation.OperationDescriptor {
	descriptors := append((&AppModel{}).taskActionOperationDescriptors(), filterOperationDescriptors()...)
	descriptors = append(descriptors, projectFilterOperationDescriptors()...)
	descriptors = append(descriptors, sortAxisOperationDescriptors()...)
	descriptors = append(descriptors, checklistRowOperationDescriptors()...)
	descriptors = append(descriptors, (&AppModel{}).checklistStartOperationDescriptor())
	descriptors = append(descriptors, projectActionSheetOperationDescriptor())
	return append(descriptors, projecthub.ProjectOperationDescriptors()...)
}

func simplifiedOperationContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	return ctx
}
