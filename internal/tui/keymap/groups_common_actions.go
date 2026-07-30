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
	// OpenURL opens the task's URL in a browser. Nil if not supported.
	OpenURL Action
	// MoveToProject opens the project-move dialog. Nil if not supported.
	MoveToProject Action
	// QuickNote opens a quick note on the selected task. Nil if not supported.
	QuickNote Action
}

// CommonTaskActionGroup returns top-level task-action bindings shared across
// primary views: o, Z, D, e, ?, and optional O / ctrl+b / ctrl+.
func CommonTaskActionGroup(deps CommonTaskActionDeps) []Entry {
	cat := "Task Actions"
	entries := []Entry{
		Bind("o", "View task details", cat, deps.ViewTask,
			WithOperationID("tui.command.view.task.details")),
		Bind("Z", "Snooze date picker", cat, deps.SnoozeDatePicker),
		Bind("D", "Mark done", cat, deps.MarkDone),
		Bind("e", "Edit task", cat, deps.EditTask,
			WithOperationID("tui.command.edit.task")),
		Bind("?", "Toggle help", cat, deps.ToggleHelp),
	}
	if deps.OpenURL != nil {
		entries = append(entries, Bind("O", "Open URL in browser", cat, deps.OpenURL))
	}
	if deps.MoveToProject != nil {
		entries = append(entries, Bind("ctrl+b", "Move task to project", cat, deps.MoveToProject,
			WithOperationID("tui.command.move.to.project")))
	}
	if deps.QuickNote != nil {
		entries = append(entries, Bind("ctrl+.", "Quick note on task", cat, deps.QuickNote,
			WithOperationID("tui.command.quick.note")))
	}
	return entries
}
