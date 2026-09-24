package filterops

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
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

func SortAxisOperationDescriptors() []operation.OperationDescriptor {
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

func (c *Controller) RegisterSortOperations(shell Shell) error {
	if shell.Registry() == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range SortAxisOperationDescriptors() {
		// Ranking is offered only while a query is held, because with no query
		// there is nothing to rank; every other order key stands unconditionally.
		if descriptor.ID == operation.OperationSortRanked {
			descriptor.Applicability = func(ctx operation.CommandContext) bool {
				return operation.CoreOutlinerApplicability(ctx) &&
					shell.Outliner() != nil && shell.Outliner().HasSearchProjection()
			}
		}
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.ExecuteSortOperation(shell, invocation)
			},
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

func (c *Controller) ExecuteSortOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	if invocation.Descriptor.ID == operation.OperationSortRanked {
		if !out.ToggleRankedList() {
			return operation.OperationHandlerResult{Status: "Ranked list needs a search · press / first"}
		}
		if out.ShowsRankedList() {
			return operation.OperationHandlerResult{Status: "Ranked list · one flat relevance order"}
		}
		return operation.OperationHandlerResult{Status: "Ranked list off · results narrow the mode"}
	}

	field, known := sortFieldOperations[invocation.Descriptor.ID]
	if !known {
		return operation.OperationHandlerResult{Err: fmt.Errorf("unknown sort operation %s", invocation.Descriptor.ID)}
	}

	if invocation.Descriptor.ID == operation.OperationSortClear {
		out.Sort.Reset()
		out.TreeSort.Reset()
		out.Population.Invalidate()
		outliner.RebuildOutlinerPreservingCursor(out)
		return operation.OperationHandlerResult{Status: "Sort cleared"}
	}

	// A fixed-order mode owns its ordering, so the key records the choice on the
	// durable tree sort and the mode transition installs it on return. A search
	// projection is flat and owns no mode order at all, so the mode underneath it
	// is irrelevant there and the sort installs directly.
	mode := out.ActiveOutlinerModeEntry()
	if mode.FixedOrder() && !out.ShowsRankedList() {
		out.TreeSort.Toggle(field)
		return operation.OperationHandlerResult{Status: fmt.Sprintf(
			"%s owns its order · sort applies in All, Inbox, Backlog", mode.Label)}
	}

	out.Sort.Toggle(field)
	out.TreeSort = out.Sort
	out.Population.Invalidate()
	outliner.RebuildOutlinerPreservingCursor(out)
	return operation.OperationHandlerResult{Status: "Sorting by " + sortStateLabel(out.Sort)}
}

func sortStateLabel(state query.SortState) string {
	direction := "↑"
	if !state.Ascending {
		direction = "↓"
	}
	return state.Field.String() + " " + direction
}
