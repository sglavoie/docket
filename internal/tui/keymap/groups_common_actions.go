package keymap

// CommonTaskActionDeps holds action closures shared across primary task views.
type CommonTaskActionDeps struct {
	// ViewTask opens the task detail view.
	ViewTask Action
	// SnoozeDatePicker opens the snooze date picker dialog.
	SnoozeDatePicker Action
	// MarkDone marks the current task as done.
	MarkDone Action
	// EditTask opens inline task editing.
	EditTask Action
	// ToggleHelp shows or hides the help panel.
	ToggleHelp Action
	// ToggleActive toggles the active flag on the task.
	ToggleActive Action
	// EditScore opens the score-dimensions editor.
	EditScore Action
	// OpenURL opens the task's URL in a browser. Nil if not supported.
	OpenURL Action
	// MoveToProject opens the project-move dialog. Nil if not supported.
	MoveToProject Action
	// QuickNote opens a quick note on the selected task. Nil if not supported.
	QuickNote Action
}

// CommonTaskActionGroup returns top-level task-action bindings shared across
// primary views: o, Z, D, e, ?, *, S, and optional O / ctrl+b / ctrl+.
func CommonTaskActionGroup(deps CommonTaskActionDeps) []Entry {
	cat := "Task Actions"
	entries := []Entry{
		Bind("o", "View task details", cat, deps.ViewTask),
		Bind("Z", "Snooze date picker", cat, deps.SnoozeDatePicker),
		Bind("D", "Mark done", cat, deps.MarkDone),
		Bind("e", "Edit task", cat, deps.EditTask),
		Bind("?", "Toggle help", cat, deps.ToggleHelp),
		Bind("*", "Toggle active flag", cat, deps.ToggleActive),
		Bind("S", "Edit score dimensions", cat, deps.EditScore),
	}
	if deps.OpenURL != nil {
		entries = append(entries, Bind("O", "Open URL in browser", cat, deps.OpenURL))
	}
	if deps.MoveToProject != nil {
		entries = append(entries, Bind("ctrl+b", "Move task to project", cat, deps.MoveToProject))
	}
	if deps.QuickNote != nil {
		entries = append(entries, Bind("ctrl+.", "Quick note on task", cat, deps.QuickNote))
	}
	return entries
}
