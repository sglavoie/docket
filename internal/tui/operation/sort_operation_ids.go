package operation

// The order-axis operation IDs. One ID per sort field rather than one
// preset-valued operation, so the axis needs no value provider and cannot grow
// a sort picker. They are `OperationID` vocabulary rather than sort state, so
// they sit beside the operation layer that owns the type instead of beside the
// field mapping the descriptors are written against.
const (
	OperationSortDueDate   OperationID = "tui.sort.due"
	OperationSortPriority  OperationID = "tui.sort.priority"
	OperationSortCreatedAt OperationID = "tui.sort.created"
	OperationSortUpdatedAt OperationID = "tui.sort.updated"
	OperationSortTitle     OperationID = "tui.sort.title"
	OperationSortClear     OperationID = "tui.sort.clear"
	OperationSortRanked    OperationID = "tui.sort.ranked"
)
