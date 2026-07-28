package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
)

// Saved-filter operation IDs. A saved filter is identified by its stable ID, so
// a filter renamed since the row was built still resolves.
const (
	operationSavedFilterApply  OperationID = "tui.saved-filter.apply"
	operationSavedFilterEdit   OperationID = "tui.saved-filter.edit"
	operationSavedFilterDelete OperationID = "tui.saved-filter.delete"
)

func savedFilterOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          OperationCategoryFilter,
			Placement:         PlacementFinderOperation,
			TargetSource:      OperationTargetCurrentEntity,
			TargetCardinality: TargetCardinalityOne,
			Applicability:     savedFilterOperationApplicable,
			Risk:              risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationSavedFilterApply, "Apply saved filter", "Apply this saved filter to the Outliner", OperationRiskReadOnly, "load filter"),
		descriptor(operationSavedFilterEdit, "Edit saved filter", "Open this saved filter in the filter builder", OperationRiskReadOnly),
		descriptor(operationSavedFilterDelete, "Delete saved filter", "Remove this saved filter", OperationRiskDestructive),
	}
}

func savedFilterOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindSavedFilter
}

func (m *AppModel) registerSavedFilterOperations() error {
	for _, descriptor := range savedFilterOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			SingleHandler:         m.executeSavedFilterOperation,
			DelegateValueProvider: true,
			DelegateTransaction:   true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// resolveSavedFilterTarget rereads the filter from planning state so a filter
// deleted since the row was built fails instead of applying a stale copy.
func (m *AppModel) resolveSavedFilterTarget(identity EntityIdentity) (*config.SavedFilter, error) {
	if identity.Name == "" {
		return nil, fmt.Errorf("saved filter target has no id")
	}
	if m.runtime == nil {
		return nil, fmt.Errorf("saved filters are unavailable")
	}
	filter := m.runtime.Planning().GetSavedFilter(identity.Name)
	if filter == nil {
		return nil, fmt.Errorf("saved filter %s is no longer available", identity.Name)
	}
	return filter, nil
}

//nolint:exhaustive // Only saved-filter operations are registered with this handler.
func (m *AppModel) executeSavedFilterOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindSavedFilter {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a saved filter target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	filter, err := m.resolveSavedFilterTarget(target.Entity)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}

	switch invocation.Descriptor.ID {
	case operationSavedFilterApply:
		return OperationHandlerResult{Message: LoadSavedFilterMsg{Filter: *filter}}

	case operationSavedFilterEdit:
		return OperationHandlerResult{Message: ShowFilterEditorMsg{Filter: filter}}

	case operationSavedFilterDelete:
		return OperationHandlerResult{Message: DeleteSavedFilterMsg{ID: filter.ID}}

	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}
