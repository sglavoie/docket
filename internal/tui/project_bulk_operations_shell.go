package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/operation"
)

func (m *AppModel) registerProjectBulkOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		operation.ValueProviderTagName,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range projectBulkOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityProjectHub),
			BulkHandler:          executeProjectBulkTags,
			// The replacement tag sets are computed against freshly resolved
			// projects and committed as one catalog transaction, which already owns
			// its undo record; parking confirms in the shell because only the
			// resolved sets know which projects a tag edit newly parks.
			DelegateConfirmation: true,
			DelegateTransaction:  true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func executeProjectBulkTags(
	invocation OperationInvocation,
	targets []operation.OperationTarget,
) OperationHandlerResult {
	message := ProjectBulkTagsMsg{
		ProjectIDs: make([]string, 0, len(targets)),
		Action:     projectBulkTagActions[invocation.Descriptor.ID],
	}
	for _, target := range targets {
		if target.Entity.Kind != operation.EntityKindProject || target.ProjectID == "" {
			return OperationHandlerResult{Err: fmt.Errorf(
				"%s needs project targets, got %s",
				invocation.Descriptor.Label,
				target.Entity,
			)}
		}
		message.ProjectIDs = append(message.ProjectIDs, target.ProjectID)
	}
	if invocation.Descriptor.ValueProviderID == "" {
		return OperationHandlerResult{Message: message}
	}
	tag, ok := invocation.Value.(string)
	if !ok {
		return OperationHandlerResult{Err: fmt.Errorf(
			"%s needs a tag name",
			invocation.Descriptor.Label,
		)}
	}
	message.Tag = tag
	return OperationHandlerResult{Message: message}
}
