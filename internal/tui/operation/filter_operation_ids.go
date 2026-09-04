package operation

// The facet operation IDs. They are `OperationID` vocabulary rather than filter
// state, so they sit beside the operation layer that owns the type instead of
// beside the clause helpers the facets are written in: the registrar, the
// simplified input strategy, and the menu bar all name them, and only the last
// of those is above the filter carriers.
const (
	OperationFilterPriority      OperationID = "tui.filter.priority"
	OperationFilterStates        OperationID = "tui.filter.states"
	OperationFilterTags          OperationID = "tui.filter.tags"
	OperationFilterContexts      OperationID = "tui.filter.contexts"
	OperationFilterReview        OperationID = "tui.filter.review"
	OperationFilterDue           OperationID = "tui.filter.due"
	OperationFilterCompletionAge OperationID = "tui.filter.completion-age"
	OperationFilterHierarchy     OperationID = "tui.filter.hierarchy"
	OperationFilterDependency    OperationID = "tui.filter.dependency"
	OperationFilterStale         OperationID = "tui.filter.stale"
	OperationFilterPinned        OperationID = "tui.filter.pinned"
	OperationFilterClear         OperationID = "tui.filter.clear"
)

// The F chord owns the population facets. They are separate operations from the
// f chord's row facets because they decide which projects load at all: applying
// one needs a reload, not a rebuild.
const (
	OperationFilterProjectTags     OperationID = "tui.filter.project-tags"
	OperationFilterShowParked      OperationID = "tui.filter.show-parked"
	OperationFilterOnlyParked      OperationID = "tui.filter.only-parked"
	OperationFilterIncludeArchived OperationID = "tui.filter.include-archived"
	OperationFilterOnlyArchived    OperationID = "tui.filter.only-archived"
	OperationFilterProjectClear    OperationID = "tui.filter.project-clear"
)
