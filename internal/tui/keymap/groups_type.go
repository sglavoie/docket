package keymap

// TypeChangeDeps holds action closures for task-type change bindings.
type TypeChangeDeps struct {
	// PromoteType promotes the task type (e.g. task → story).
	PromoteType Action
	// DemoteType demotes the task type (e.g. story → task).
	DemoteType Action
	// CycleLeafLeft cycles leaf types to the left.
	CycleLeafLeft Action
	// CycleLeafRight cycles leaf types to the right.
	CycleLeafRight Action
}

// TypeChangeGroup returns top-level type-change bindings: <, >, H, L.
func TypeChangeGroup(deps TypeChangeDeps) []Entry {
	cat := "Type Changes"
	return []Entry{
		Bind("<", "Promote type", cat, deps.PromoteType),
		Bind(">", "Demote type", cat, deps.DemoteType),
		Bind("H", "Cycle leaf left", cat, deps.CycleLeafLeft),
		Bind("L", "Cycle leaf right", cat, deps.CycleLeafRight),
	}
}
