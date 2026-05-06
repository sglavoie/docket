package keymap

// SortDeps holds action closures for the sort-order bindings.
type SortDeps struct {
	// SortByPriority sorts by priority.
	SortByPriority Action
	// SortByDueDate sorts by due date.
	SortByDueDate Action
	// SortByCreated sorts by creation date.
	SortByCreated Action
	// SortByStatus sorts by status.
	SortByStatus Action
	// SortByScore sorts by score.
	SortByScore Action
	// ResetSort resets to default sort order.
	ResetSort Action
}

// SortGroup returns top-level sort bindings: 1-5 for specific sorts, 0 to reset.
func SortGroup(deps SortDeps) []Entry {
	cat := "Sort"
	return []Entry{
		Bind("1", "Sort by priority", cat, deps.SortByPriority),
		Bind("2", "Sort by due date", cat, deps.SortByDueDate),
		Bind("3", "Sort by created", cat, deps.SortByCreated),
		Bind("4", "Sort by status", cat, deps.SortByStatus),
		Bind("5", "Sort by score", cat, deps.SortByScore),
		Bind("0", "Reset sort", cat, deps.ResetSort),
	}
}
