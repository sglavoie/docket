package keymap

// TaskPrefixDeps holds action closures for the "t" task-action prefix group.
type TaskPrefixDeps struct {
	// NewTask creates a new task (no template picker).
	NewTask Action
	// NewTaskWithTemplate creates a new task with template picker.
	NewTaskWithTemplate Action
	// RenameTask renames the current task inline.
	RenameTask Action
	// ChangeSection opens the section picker for the current task.
	ChangeSection Action
	// OpenEditor opens the task in an external editor.
	OpenEditor Action
	// TogglePin toggles the pinned flag.
	TogglePin Action
	// ToggleDim toggles the dimmed flag.
	ToggleDim Action
	// ManageTags opens the tag selector.
	ManageTags Action
	// ViewComments opens the task comments view.
	ViewComments Action
	// ViewAttachments opens the task attachments view.
	ViewAttachments Action
	// EditChecklist opens the task checklist editor.
	EditChecklist Action
	// EditDescription opens the description editor.
	EditDescription Action
	// MoveToProject opens the project selector for moving the task.
	MoveToProject Action
	// ViewDecisions opens the task decisions view.
	ViewDecisions Action
	// DeleteTask deletes the current task (may show confirm dialog).
	DeleteTask Action
}

// TaskPrefixGroup returns entries for the "t" prefix: task operations.
func TaskPrefixGroup(deps TaskPrefixDeps) []Entry {
	cat := "Task (t prefix)"
	children := []Entry{
		Bind("n", "New task (no template)", cat, deps.NewTask),
		Bind("N", "New task (with template picker)", cat, deps.NewTaskWithTemplate),
		Bind("r", "Rename task (inline)", cat, deps.RenameTask),
		Bind("s", "Change section", cat, deps.ChangeSection),
		Bind("o", "Open in external editor", cat, deps.OpenEditor),
		Bind("p", "Pin/unpin task", cat, deps.TogglePin),
		Bind("h", "Dim/undim task", cat, deps.ToggleDim),
		Bind("t", "Manage tags", cat, deps.ManageTags),
		Bind("c", "View/add comments", cat, deps.ViewComments),
		Bind("a", "View/add attachments", cat, deps.ViewAttachments),
		Bind("l", "Edit checklist", cat, deps.EditChecklist),
		Bind("e", "Edit description", cat, deps.EditDescription),
		Bind("m", "Move to another project", cat, deps.MoveToProject),
		Bind("d", "View/add decisions", cat, deps.ViewDecisions),
		Bind("D", "Delete task", cat, deps.DeleteTask),
	}
	return []Entry{Prefix("t", "Task actions", cat, children)}
}
