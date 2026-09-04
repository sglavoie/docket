package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// sortFieldOperations maps each order-axis operation to the field it installs.
// One descriptor per field rather than one preset-valued operation, so the axis
// needs no value provider and cannot grow a sort picker.
var sortFieldOperations = map[operation.OperationID]query.SortField{
	operation.OperationSortDueDate:   query.SortByDueDate,
	operation.OperationSortPriority:  query.SortByPriority,
	operation.OperationSortCreatedAt: query.SortByCreatedAt,
	operation.OperationSortUpdatedAt: query.SortByUpdatedAt,
	operation.OperationSortTitle:     query.SortByTitle,
	operation.OperationSortClear:     query.SortByDefault,
}

// sortOperationDescriptor is the shape every order-axis key shares: no target,
// no value, no transaction, and applicability decided by the Outliner alone.
func sortOperationDescriptor(id operation.OperationID, label string, aliases ...string) operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                   id,
		Label:                label,
		Aliases:              aliases,
		Category:             operation.OperationCategorySort,
		TargetCardinality:    operation.TargetCardinalityNone,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Applicability:        operation.CoreOutlinerApplicability,
		Risk:                 operation.OperationRiskReadOnly,
	}
}

func sortAxisOperationDescriptors() []operation.OperationDescriptor {
	descriptor := sortOperationDescriptor
	return []operation.OperationDescriptor{
		descriptor(operation.OperationSortDueDate, "Sort by Due Date", "order by due date"),
		descriptor(operation.OperationSortPriority, "Sort by Priority", "order by priority"),
		descriptor(operation.OperationSortCreatedAt, "Sort by Created", "order by creation date"),
		descriptor(operation.OperationSortUpdatedAt, "Sort by Updated", "order by last touch"),
		descriptor(operation.OperationSortTitle, "Sort by Title", "order alphabetically"),
		descriptor(operation.OperationSortClear, "Clear Sort", "restore the mode's own order"),
		// The flat ranked list is one more ordering of the rows the held query
		// already selected, not a mode and not a second way to search, which is why
		// it sits on the order axis rather than on a root key of its own.
		descriptor(operation.OperationSortRanked, "Toggle Ranked List", "flat relevance order"),
	}
}

func (m *AppModel) registerSortOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range sortAxisOperationDescriptors() {
		// Ranking is offered only while a query is held, because with no query
		// there is nothing to rank; every other order key stands unconditionally.
		if descriptor.ID == operation.OperationSortRanked {
			descriptor.Applicability = func(ctx operation.CommandContext) bool {
				return operation.CoreOutlinerApplicability(ctx) &&
					m.crossOutliner != nil && m.crossOutliner.hasSearchProjection()
			}
		}
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
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

func (m *AppModel) executeSortOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	if invocation.Descriptor.ID == operation.OperationSortRanked {
		if !out.toggleRankedList() {
			return operation.OperationHandlerResult{Status: "Search for something first"}
		}
		if out.showsRankedList() {
			return operation.OperationHandlerResult{Status: "Ranked list · one flat relevance order"}
		}
		return operation.OperationHandlerResult{Status: "Ranked list off · results narrow the mode"}
	}

	field, known := sortFieldOperations[invocation.Descriptor.ID]
	if !known {
		return operation.OperationHandlerResult{Err: fmt.Errorf("unknown sort operation %s", invocation.Descriptor.ID)}
	}

	if invocation.Descriptor.ID == operation.OperationSortClear {
		out.sort.Reset()
		out.treeSort.Reset()
		out.population.invalidate()
		rebuildOutlinerPreservingCursor(out)
		return operation.OperationHandlerResult{Status: "Sort cleared"}
	}

	// A fixed-order mode owns its ordering, so the key records the choice on the
	// durable tree sort and the mode transition installs it on return. A search
	// projection is flat and owns no mode order at all, so the mode underneath it
	// is irrelevant there and the sort installs directly.
	mode := out.activeOutlinerModeEntry()
	if mode.FixedOrder() && !out.showsRankedList() {
		out.treeSort.Toggle(field)
		return operation.OperationHandlerResult{Status: fmt.Sprintf(
			"%s owns its order · sort applies in All, Inbox, Backlog", mode.Label)}
	}

	out.sort.Toggle(field)
	out.treeSort = out.sort
	out.population.invalidate()
	rebuildOutlinerPreservingCursor(out)
	status := "Sorting by " + sortStateLabel(out.sort)
	if exempt := out.manualOrderSectionNames(); len(exempt) > 0 {
		status += " · " + strings.Join(exempt, ", ") + " keep manual order"
	}
	return operation.OperationHandlerResult{Status: status}
}

func sortStateLabel(state query.SortState) string {
	direction := "↑"
	if !state.Ascending {
		direction = "↓"
	}
	return state.Field.String() + " " + direction
}
