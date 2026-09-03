package tui

// The facet operation IDs. They are `OperationID` vocabulary rather than filter
// state, so they sit beside the operation layer that owns the type instead of
// beside the clause helpers the facets are written in: the registrar, the
// simplified input strategy, and the menu bar all name them, and only the last
// of those is above the filter carriers.
const (
	operationFilterPriority      OperationID = "tui.filter.priority"
	operationFilterStates        OperationID = "tui.filter.states"
	operationFilterTags          OperationID = "tui.filter.tags"
	operationFilterContexts      OperationID = "tui.filter.contexts"
	operationFilterReview        OperationID = "tui.filter.review"
	operationFilterDue           OperationID = "tui.filter.due"
	operationFilterCompletionAge OperationID = "tui.filter.completion-age"
	operationFilterHierarchy     OperationID = "tui.filter.hierarchy"
	operationFilterDependency    OperationID = "tui.filter.dependency"
	operationFilterStale         OperationID = "tui.filter.stale"
	operationFilterPinned        OperationID = "tui.filter.pinned"
	operationFilterClear         OperationID = "tui.filter.clear"
)
