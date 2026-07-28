package tui

import "fmt"

// operationModeActivate is the primary action a mode result offers. It is
// contextual rather than Finder-listed because the Modes category already lists
// every mode once, and Enter on that row means the same thing.
const operationModeActivate OperationID = "tui.mode.activate"

func modeOperationDescriptors() []OperationDescriptor {
	return []OperationDescriptor{{
		ID:                operationModeActivate,
		Label:             "Activate mode",
		Description:       "Switch the Outliner to this mode",
		Aliases:           []string{"switch mode", "open mode"},
		Category:          OperationCategoryMode,
		Placement:         PlacementContextual,
		TargetSource:      OperationTargetCurrentEntity,
		TargetCardinality: TargetCardinalityOne,
		Applicability:     modeOperationApplicable,
		Risk:              OperationRiskReadOnly,
	}}
}

func modeOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindMode
}

func (m *AppModel) registerModeOperations() error {
	for _, descriptor := range modeOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			SingleHandler:         m.executeModeOperation,
			DelegateValueProvider: true,
			DelegateTransaction:   true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeModeOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindMode {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a mode target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	mode, ok := OutlinerModeByID(OutlinerModeID(target.Entity.Name))
	if !ok {
		return OperationHandlerResult{Err: fmt.Errorf("mode %s is no longer available", target.Entity.Name)}
	}
	return OperationHandlerResult{Message: ActivateOutlinerModeMsg{Mode: mode.ID}}
}
