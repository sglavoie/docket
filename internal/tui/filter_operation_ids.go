package tui

import "github.com/sglavoie/tt/internal/tui/operation"

// The facet operation IDs. They are `OperationID` vocabulary rather than filter
// state, so they sit beside the operation layer that owns the type instead of
// beside the clause helpers the facets are written in: the registrar, the
// simplified input strategy, and the menu bar all name them, and only the last
// of those is above the filter carriers.
const (
	operationFilterPriority      operation.OperationID = "tui.filter.priority"
	operationFilterStates        operation.OperationID = "tui.filter.states"
	operationFilterTags          operation.OperationID = "tui.filter.tags"
	operationFilterContexts      operation.OperationID = "tui.filter.contexts"
	operationFilterReview        operation.OperationID = "tui.filter.review"
	operationFilterDue           operation.OperationID = "tui.filter.due"
	operationFilterCompletionAge operation.OperationID = "tui.filter.completion-age"
	operationFilterHierarchy     operation.OperationID = "tui.filter.hierarchy"
	operationFilterDependency    operation.OperationID = "tui.filter.dependency"
	operationFilterStale         operation.OperationID = "tui.filter.stale"
	operationFilterPinned        operation.OperationID = "tui.filter.pinned"
	operationFilterClear         operation.OperationID = "tui.filter.clear"
)
