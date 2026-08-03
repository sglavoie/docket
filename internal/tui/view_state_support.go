package tui

// UndoContext tracks the action label and description for the next undo snapshot.
// Embed this in views that support undo to avoid duplicating fields and methods.
type UndoContext struct {
	Action  string   // e.g. "Complete Task", "Set Priority"
	Context string   // e.g. task title or "3 tasks"
	TaskIDs []string // Task IDs explicitly changed by user (enables targeted undo)
}

// Set records the action and context for the next undo snapshot. Targeted
// TaskIDs/ProjectIDs are cleared because each new operation starts fresh;
// callers that need targeting assign those fields explicitly after Set.
func (u *UndoContext) Set(action, ctx string) {
	u.Action = action
	u.Context = ctx
	u.TaskIDs = nil
}

// Clear resets the undo context after it has been consumed.
func (u *UndoContext) Clear() {
	u.Action = ""
	u.Context = ""
	u.TaskIDs = nil
}
