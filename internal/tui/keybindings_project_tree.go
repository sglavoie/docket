package tui

func (r *KeybindingRegistry) registerProjectTreeBindings() {
	views := []ViewType{ViewProjectTree}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Switch section (Active/Next/Inbox/Waiting/Someday)", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "View task details", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Expand/collapse", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Cycle sidebar (hide/view/focus)", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Jump to task", Category: "Navigation", Views: views},
		Keybinding{Key: "n", Description: "Jump to next root task", Category: "Navigation", Views: views},
		Keybinding{Key: "N", Description: "Jump to previous root task", Category: "Navigation", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Navigation", Views: views},
		Keybinding{Key: "f", Description: "Focus view (project)", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},

		// Sidebar
		Keybinding{Key: "j/k (sidebar)", Description: "Scroll/navigate dependencies", Category: "Sidebar", Views: views},
		Keybinding{Key: "h/l (sidebar)", Description: "Switch dependency section", Category: "Sidebar", Views: views},
		Keybinding{Key: "Enter (sidebar)", Description: "Jump to dependency", Category: "Sidebar", Views: views},
		Keybinding{Key: "Esc (sidebar)", Description: "Return focus to tree", Category: "Sidebar", Views: views},

		// Snooze Quick Actions (s prefix)
		Keybinding{Key: "st", Description: "Snooze until tomorrow", Category: "Snooze", Views: views},
		Keybinding{Key: "sw", Description: "Snooze 1 week", Category: "Snooze", Views: views},
		Keybinding{Key: "sm", Description: "Snooze 1 month", Category: "Snooze", Views: views},
		Keybinding{Key: "sn", Description: "Snooze until next Monday", Category: "Snooze", Views: views},
		Keybinding{Key: "s0", Description: "Clear snooze", Category: "Snooze", Views: views},
		Keybinding{Key: "sd", Description: "Snooze date picker", Category: "Snooze", Views: views},
		Keybinding{Key: "gz", Description: "Go to Snoozed view", Category: "Views", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Open outliner view", Category: "Views", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Open outliner view", Category: "Views", Views: views},
		Keybinding{Key: "A", Description: "All tasks (normal mode)", Category: "Views", Views: views},

		// Task Movement (when sidebar unfocused)
		Keybinding{Key: "K", Description: "Move task up", Category: "Task Movement", Views: views},
		Keybinding{Key: "J", Description: "Move task down", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Legacy Task Operations (still available for compatibility)
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "o", Description: "View task details", Category: "Task Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Task Operations", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Task Operations", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+M", Description: "Move/reparent task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+;", Description: "Toggle focus mode (zoom into subtree)", Category: "Task Operations", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Task Operations", Views: views},
		Keybinding{Key: "gl", Description: "Project recent activity", Category: "Navigation", Views: views},

		Keybinding{Key: "cx", Description: "Unlink linked task", Category: "Linked Tasks", Views: views},
		Keybinding{Key: "x (sidebar)", Description: "Remove link", Category: "Linked Tasks", Views: views},

		// Project Views
		Keybinding{Key: "B", Description: "Project blockers", Category: "Project Views", Views: views},

		// Views (g prefix)
		Keybinding{Key: "gv", Description: "Cycle view (Active/Next/Inbox/Waiting/Someday/Completed/Snoozed/All)", Category: "Views", Views: views},
		Keybinding{Key: "ga", Description: "Go to Active view", Category: "Views", Views: views},
		Keybinding{Key: "gn", Description: "Go to Next view", Category: "Views", Views: views},
		Keybinding{Key: "gi", Description: "Go to Inbox view", Category: "Views", Views: views},
		Keybinding{Key: "gw", Description: "Go to Waiting view", Category: "Views", Views: views},
		Keybinding{Key: "gs", Description: "Go to Someday view", Category: "Views", Views: views},
		Keybinding{Key: "gc", Description: "Go to Completed view", Category: "Views", Views: views},
		Keybinding{Key: "g*", Description: "Go to All view", Category: "Views", Views: views},
		Keybinding{Key: "go", Description: "Jump to context node's home section", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Add to today's plan", Category: "Views", Views: views},
		Keybinding{Key: "W", Description: "Inbox processor", Category: "Views", Views: views},

		// Filters
		Keybinding{Key: "C", Description: "Filter by context", Category: "Filters", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Filters", Views: views},
		Keybinding{Key: "Fp", Description: "Filter by priority (use Fp)", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "gd", Description: "Cycle due date filter", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Filter summary", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Custom section manager", Category: "Filters", Views: views},
		Keybinding{Key: "Fvs", Description: "Save current view", Category: "Filters", Views: views},
		Keybinding{Key: "Fvl", Description: "Load saved filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fvd", Description: "Delete saved filter", Category: "Filters", Views: views},

		// Scoring
		Keybinding{Key: "S", Description: "Score editor", Category: "Scoring", Views: views},
		Keybinding{Key: "FS", Description: "Score threshold filter", Category: "Scoring", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Focus mode (zoom into subtree)", Category: "Navigation", Views: views},
		Keybinding{Key: "a", Description: "Select all visible (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "A", Description: "Deselect all visible (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "gX", Description: "Deselect all (in multi-select)", Category: "Multi-select", Views: views},

		// Project
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},
		Keybinding{Key: "ctrl+e", Description: "Export project to Markdown", Category: "Project", Views: views},

		// Filter Popup (F prefix)
		Keybinding{Key: "F", Description: "Open filter popup", Category: "Filters", Views: views},
		Keybinding{Key: "Fc", Description: "Toggle completed view", Category: "Filters", Views: views},
		Keybinding{Key: "Fp", Description: "Toggle high priority filter", Category: "Filters", Views: views},
		Keybinding{Key: "FP", Description: "Filter by project (cross-project)", Category: "Filters", Views: views},
		Keybinding{Key: "Ft", Description: "Filter by type (sub-popup, use Ft since t is task prefix)", Category: "Filters", Views: views},
		Keybinding{Key: "Fs", Description: "Filter by status (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "F#", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "F@", Description: "Filter by section", Category: "Filters", Views: views},
		Keybinding{Key: "Fd", Description: "Filter by due date (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fa", Description: "Filter by attribute (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fr", Description: "Toggle actionable/ready filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fh", Description: "Toggle hide dimmed tasks", Category: "Filters", Views: views},
		Keybinding{Key: "Fx", Description: "Clear all filters", Category: "Filters", Views: views},

		// Custom Section Tabs
		Keybinding{Key: "[/]", Description: "Cycle custom section tabs", Category: "Navigation", Views: views},

		// Notes
		Keybinding{Key: "{", Description: "Open notes view", Category: "Notes", Views: views},
		Keybinding{Key: "}", Description: "Create new note", Category: "Notes", Views: views},

		// Recurring Tasks
		Keybinding{Key: "n / e", Description: "Set recurrence in task creator/editor", Category: "Recurring Tasks", Views: views},
		Keybinding{Key: "i + ~", Description: "Quick capture: ~daily, ~weekly, ~2w, etc.", Category: "Recurring Tasks", Views: views},
		Keybinding{Key: ",d", Description: "Complete recurring task (creates next instance)", Category: "Recurring Tasks", Views: views},

		// Archive
		Keybinding{Key: "A", Description: "Archive completed (Completed view only)", Category: "Archive", Views: views},

		// Other
		Keybinding{Key: "R", Description: "Reload from disk", Category: "Other", Views: views},
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+r", Description: "Redo", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+v", Description: "Create task from clipboard (nav mode) / paste text (search)", Category: "Task Operations", Views: views},
	)
	r.addForViews(views, foldBindings())
	r.addForViews(views, typeChangeBindings())
	r.addForViews(views, taskPrefixBindings())
	r.addForViews(views, blockerPrefixBindings())
	r.addForViews(views, movePrefixBindings())
	r.addForViews(views, sortBindings())
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, clipboardBindings())
	r.addForViews(views, linkedTaskBindings())
	r.addForViews(views, projectCyclingBindings())
}
