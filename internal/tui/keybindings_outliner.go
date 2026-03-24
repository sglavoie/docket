package tui

func (r *KeybindingRegistry) registerOutlinerBindings() {
	views := []ViewType{ViewOutliner}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+d/u", Description: "Page down/up", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Zoom into parent / open leaf", Category: "Navigation", Views: views},
		Keybinding{Key: "Backspace", Description: "Zoom out one level", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Zoom out / close", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Jump to task", Category: "Navigation", Views: views},
		Keybinding{Key: "n", Description: "Jump to next section", Category: "Navigation", Views: views},
		Keybinding{Key: "N", Description: "Jump to previous section", Category: "Navigation", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Navigation", Views: views},
		Keybinding{Key: "gz", Description: "Jump to Snoozed perspective", Category: "Navigation", Views: views},
		Keybinding{Key: "gl", Description: "Project recent activity", Category: "Navigation", Views: views},

		// Perspectives
		Keybinding{Key: "h/l", Description: "Cycle perspective left/right", Category: "Perspectives", Views: views},
		Keybinding{Key: "P0-P9", Description: "Switch perspective by number", Category: "Perspectives", Views: views},

		// View switching
		Keybinding{Key: "ctrl+P", Description: "Project tree view", Category: "Views", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Filter summary", Category: "Views", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "Focus view (project)", Category: "Views", Views: views},
		Keybinding{Key: "A", Description: "All tasks / archive (Completed)", Category: "Views", Views: views},
		Keybinding{Key: "B", Description: "Project blockers", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Inbox processor", Category: "Views", Views: views},

		// Task Movement
		Keybinding{Key: "J", Description: "Move task down", Category: "Task Movement", Views: views},
		Keybinding{Key: "K", Description: "Move task up", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Editing
		Keybinding{Key: "o", Description: "View task details", Category: "Editing", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Editing", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Editing", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Editing", Views: views},

		// Task operations
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+M", Description: "Move/reparent task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},

		// Search
		Keybinding{Key: "/", Description: "Search tasks", Category: "Search", Views: views},

		// Filters
		Keybinding{Key: "F", Description: "Open filter popup", Category: "Filters", Views: views},
		Keybinding{Key: "Fp", Description: "Toggle high priority filter", Category: "Filters", Views: views},
		Keybinding{Key: "Ft", Description: "Filter by type (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fs", Description: "Filter by status (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "F#", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "Fd", Description: "Filter by due date (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fa", Description: "Filter by attribute (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fr", Description: "Toggle actionable/ready filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fc", Description: "Toggle completed view", Category: "Filters", Views: views},
		Keybinding{Key: "F@", Description: "Section filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "Fx", Description: "Clear all filters", Category: "Filters", Views: views},
		Keybinding{Key: "FS", Description: "Score threshold filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fvs", Description: "Save current view", Category: "Filters", Views: views},
		Keybinding{Key: "Fvl", Description: "Load saved filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fvd", Description: "Delete saved filter", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Tag filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "C", Description: "Context filter", Category: "Filters", Views: views},
		Keybinding{Key: "gd", Description: "Cycle due date filter", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Filters", Views: views},
		Keybinding{Key: "[/]", Description: "Cycle custom section tabs", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Custom section manager", Category: "Filters", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-Select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle selection", Category: "Multi-Select", Views: views},
		Keybinding{Key: "a/A", Description: "Select all / deselect all", Category: "Multi-Select", Views: views},
		Keybinding{Key: "gX", Description: "Deselect all (in multi-select)", Category: "Multi-Select", Views: views},

		// Scoring
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Scoring", Views: views},

		// Notes
		Keybinding{Key: "{", Description: "Open notes view", Category: "Notes", Views: views},
		Keybinding{Key: "}", Description: "Create new note", Category: "Notes", Views: views},

		// Project
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},

		// Sidebar
		Keybinding{Key: "Tab", Description: "Cycle sidebar (hide/view/focus)", Category: "Sidebar", Views: views},
		Keybinding{Key: "j / ↓", Description: "Next item (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "k / ↑", Description: "Prev item (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "h / ←", Description: "Prev section (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "l / →", Description: "Next section (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "Enter", Description: "Jump to task (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "x", Description: "Unlink task (sidebar)", Category: "Sidebar", Views: views},
		Keybinding{Key: "Esc", Description: "Unfocus sidebar", Category: "Sidebar", Views: views},

		// Other
		Keybinding{Key: "ctrl+R", Description: "Refresh from disk", Category: "Other", Views: views},
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+Z", Description: "Undo history", Category: "Other", Views: views},
	)

	// Shared bindings
	r.addForViews(views, foldBindings())
	r.addForViews(views, sortBindings())
	r.addForViews(views, movePrefixBindings())
	r.addForViews(views, statusBindings())
	r.addForViews(views, clipboardBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, blockerPrefixBindings())
	r.addForViews(views, linkedTaskBindings())
	r.addForViews(views, typeChangeBindings())
	r.addForViews(views, taskPrefixBindings())
	r.addForViews(views, projectCyclingBindings())

	// Snooze (s prefix)
	r.add(
		Keybinding{Key: "st", Description: "Snooze until tomorrow", Category: "Snooze (s prefix)", Views: views},
		Keybinding{Key: "sw", Description: "Snooze 1 week", Category: "Snooze (s prefix)", Views: views},
		Keybinding{Key: "sm", Description: "Snooze 1 month", Category: "Snooze (s prefix)", Views: views},
		Keybinding{Key: "sn", Description: "Snooze until next Monday", Category: "Snooze (s prefix)", Views: views},
		Keybinding{Key: "s0", Description: "Clear snooze", Category: "Snooze (s prefix)", Views: views},
		Keybinding{Key: "sd", Description: "Snooze date picker", Category: "Snooze (s prefix)", Views: views},
	)
}

func (r *KeybindingRegistry) registerCrossProjectOutlinerBindings() {
	views := []ViewType{ViewCrossProjectOutliner}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+d/u", Description: "Page down/up", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Zoom into parent / open leaf", Category: "Navigation", Views: views},
		Keybinding{Key: "Backspace", Description: "Zoom out one level", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Zoom out / close", Category: "Navigation", Views: views},
		Keybinding{Key: "go", Description: "Drill down to project outliner", Category: "Navigation", Views: views},

		// Perspectives
		Keybinding{Key: "h/l", Description: "Cycle perspective left/right", Category: "Perspectives", Views: views},
		Keybinding{Key: "P0-P9", Description: "Switch perspective by number", Category: "Perspectives", Views: views},

		// Filters
		Keybinding{Key: "C", Description: "Open context picker", Category: "Filters", Views: views},

		// Search
		Keybinding{Key: "/", Description: "Search tasks", Category: "Search", Views: views},

		// Other
		Keybinding{Key: "ctrl+R", Description: "Refresh from disk", Category: "Other", Views: views},
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
	)
}
