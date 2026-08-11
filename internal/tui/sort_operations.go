package tui

import (
	"fmt"
)

const (
	operationSortDueDate   OperationID = "tui.sort.due"
	operationSortPriority  OperationID = "tui.sort.priority"
	operationSortCreatedAt OperationID = "tui.sort.created"
	operationSortUpdatedAt OperationID = "tui.sort.updated"
	operationSortTitle     OperationID = "tui.sort.title"
	operationSortClear     OperationID = "tui.sort.clear"
)

// sortFieldOperations maps each order-axis operation to the field it installs.
// One descriptor per field rather than one preset-valued operation, so the axis
// needs no value provider and cannot grow a sort picker.
var sortFieldOperations = map[OperationID]SortField{
	operationSortDueDate:   SortByDueDate,
	operationSortPriority:  SortByPriority,
	operationSortCreatedAt: SortByCreatedAt,
	operationSortUpdatedAt: SortByUpdatedAt,
	operationSortTitle:     SortByTitle,
	operationSortClear:     SortByDefault,
}

// sortOperationDescriptor is the shape every order-axis key shares: no target,
// no value, no transaction, and applicability decided by the Outliner alone.
func sortOperationDescriptor(id OperationID, label string, aliases ...string) OperationDescriptor {
	return OperationDescriptor{
		ID:                   id,
		Label:                label,
		Aliases:              aliases,
		Category:             OperationCategorySort,
		TargetCardinality:    TargetCardinalityNone,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Applicability:        coreOutlinerApplicability,
		Risk:                 OperationRiskReadOnly,
	}
}

func sortAxisOperationDescriptors() []OperationDescriptor {
	descriptor := sortOperationDescriptor
	return []OperationDescriptor{
		descriptor(operationSortDueDate, "Sort by Due Date", "order by due date"),
		descriptor(operationSortPriority, "Sort by Priority", "order by priority"),
		descriptor(operationSortCreatedAt, "Sort by Created", "order by creation date"),
		descriptor(operationSortUpdatedAt, "Sort by Updated", "order by last touch"),
		descriptor(operationSortTitle, "Sort by Title", "order alphabetically"),
		descriptor(operationSortClear, "Clear Sort", "restore the mode's own order"),
	}
}

func (m *AppModel) registerSortOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range sortAxisOperationDescriptors() {
		descriptor.Applicability = func(ctx CommandContext) bool {
			return coreOutlinerApplicability(ctx) &&
				(m.crossOutliner == nil || !m.crossOutliner.hasSearchProjection())
		}
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:            descriptor.Applicability,
			Handler:                  m.executeSortOperation,
			DelegateTargetResolution: true,
			DelegateConfirmation:     true,
			// The order axis mutates session view state only, so no workspace
			// transaction is opened.
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeSortOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	if out.hasSearchProjection() {
		return OperationHandlerResult{Status: "Exit search before changing the sort"}
	}
	field, known := sortFieldOperations[invocation.Descriptor.ID]
	if !known {
		return OperationHandlerResult{Err: fmt.Errorf("unknown sort operation %s", invocation.Descriptor.ID)}
	}

	if invocation.Descriptor.ID == operationSortClear {
		out.sort.Reset()
		out.treeSort.Reset()
		out.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(out)
		return OperationHandlerResult{Status: "Sort cleared"}
	}

	// A fixed-order mode owns its ordering, so the key records the choice on the
	// durable tree sort and the mode transition installs it on return.
	mode := out.activeOutlinerModeEntry()
	if mode.FixedOrder() {
		out.treeSort.Toggle(field)
		return OperationHandlerResult{Status: fmt.Sprintf(
			"%s owns its order · sort applies in All, Inbox, Backlog", mode.Label)}
	}

	out.sort.Toggle(field)
	out.treeSort = out.sort
	out.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(out)
	return OperationHandlerResult{Status: "Sorting by " + sortStateLabel(out.sort)}
}

func sortStateLabel(state SortState) string {
	direction := "↑"
	if !state.Ascending {
		direction = "↓"
	}
	return state.Field.String() + " " + direction
}
