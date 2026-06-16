package keymap

// CommonPrimaryViewDeps holds action closures shared across primary views
// (ProjectTree, Outliner, etc.).
type CommonPrimaryViewDeps struct {
	// OpenProjectTree opens or toggles the project tree view.
	OpenProjectTree Action
	// OpenProjectCyclePicker opens the numbered project cycle picker.
	OpenProjectCyclePicker Action
	// PrevProject cycles to the previous project.
	PrevProject Action
	// OpenCycleManager opens the cycle manager popup.
	OpenCycleManager Action
	// ProjectBlockers opens the project blockers view.
	ProjectBlockers Action
	// PriorityView opens the priority view.
	PriorityView Action
	// AddToPlan adds the task to today's plan.
	AddToPlan Action
	// InboxProcessor opens the inbox processor.
	InboxProcessor Action
	// QuickCapture opens the quick capture dialog.
	QuickCapture Action
	// BulkCapture opens the bulk capture dialog.
	BulkCapture Action
	// JumpToTask opens the task jump/search dialog.
	JumpToTask Action
	// OpenNotesView opens the notes view.
	OpenNotesView Action
	// CreateNote creates a new note.
	CreateNote Action
	// ReloadFromDisk reloads project data from disk.
	ReloadFromDisk Action
	// Redo re-applies the last undone change.
	Redo Action
	// EditLastModified opens the last-modified task for editing. Nil if not supported.
	EditLastModified Action
	// Undo undoes the last change.
	Undo Action
	// MoveReparent opens the reparent/move dialog. Nil if not supported.
	MoveReparent Action
}

// CommonPrimaryViewGroup returns top-level bindings shared across primary views.
func CommonPrimaryViewGroup(deps CommonPrimaryViewDeps) []Entry {
	cat := "Views"
	entries := []Entry{
		Bind("ctrl+p", "Open project tree view", cat, deps.OpenProjectTree),
		Bind("`", "Open project picker", "Project Cycling", deps.OpenProjectCyclePicker),
		Bind("~", "Previous project in cycle", "Project Cycling", deps.PrevProject),
		Bind("@", "Open cycle manager", "Project Cycling", deps.OpenCycleManager),
		Bind("B", "Project blockers", cat, deps.ProjectBlockers),
		Bind("p", "Priority view", cat, deps.PriorityView),
		Bind("w", "Open today's plan fallback", cat, deps.AddToPlan, AsHidden()),
		Bind("W", "Inbox processor", cat, deps.InboxProcessor),
		Bind("i", "Quick capture", "Task Operations", deps.QuickCapture),
		Bind("I", "Bulk capture", "Task Operations", deps.BulkCapture),
		Bind(`\`, "Jump to task", "Navigation", deps.JumpToTask),
		Bind("ctrl+n", "Open project Notes", "Notes", deps.OpenNotesView),
		Bind("{", "Open project Notes", "Notes", deps.OpenNotesView, AsHidden()),
		Bind("}", "Create new note", "Notes", deps.CreateNote),
		Bind("R", "Reload from disk", "Other", deps.ReloadFromDisk),
		Bind("ctrl+r", "Redo", "Other", deps.Redo),
		Bind("u", "Undo last change", "Other", deps.Undo),
	}
	if deps.EditLastModified != nil {
		entries = append(entries, Bind("ctrl+l", "Edit last modified task", "Task Operations", deps.EditLastModified))
	}
	if deps.MoveReparent != nil {
		entries = append(entries, Bind("ctrl+m", "Move/reparent/clear parent", "Task Operations", deps.MoveReparent))
	}
	return entries
}
