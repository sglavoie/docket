package tui

func (r *KeybindingRegistry) registerKanbanBindings() {
	views := []ViewType{ViewKanban}
	r.add(
		// Column Navigation
		Keybinding{Key: "h/l", Description: "Navigate between columns", Category: "Navigation", Views: views},

		// Task Navigation
		Keybinding{Key: "j/k", Description: "Navigate tasks within column", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate tasks within column", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to first/last task in column", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Jump to task", Category: "Navigation", Views: views},

		// Split View
		Keybinding{Key: "Tab", Description: "Toggle focus between kanban/details", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k (details)", Description: "Scroll description", Category: "Details Pane", Views: views},
		Keybinding{Key: "Enter (details)", Description: "View task details", Category: "Details Pane", Views: views},
		Keybinding{Key: "Esc (details)", Description: "Return to kanban", Category: "Details Pane", Views: views},

		// Task Movement Between Columns
		Keybinding{Key: "{", Description: "Move task to left column", Category: "Task Movement", Views: views},
		Keybinding{Key: "}", Description: "Move task to right column", Category: "Task Movement", Views: views},

		// Task Reordering Within Column
		Keybinding{Key: "J", Description: "Move task down in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "K", Description: "Move task up in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Group-By / Swimlanes
		Keybinding{Key: "Ctrl+G", Description: "Cycle group-by (Section/Status/Priority/Type)", Category: "Views", Views: views},
		Keybinding{Key: "Ctrl+H", Description: "Cycle swimlane mode", Category: "Views", Views: views},
		Keybinding{Key: "Ctrl+J", Description: "Next swimlane row (when active)", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+K", Description: "Previous swimlane row / close kanban", Category: "Navigation", Views: views},

		// Views
		Keybinding{Key: "gc", Description: "Completed view", Category: "Views", Views: views},
		Keybinding{Key: "g*", Description: "All view (all sections in one list)", Category: "Views", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Views", Views: views},
		Keybinding{Key: "A", Description: "All tasks", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Inbox processor", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "Focus on subtree (alias for zv)", Category: "Views", Views: views},

		// Section Navigation (g prefix)
		Keybinding{Key: "ga", Description: "Go to Active column", Category: "Sections", Views: views},
		Keybinding{Key: "gn", Description: "Go to Next column", Category: "Sections", Views: views},
		Keybinding{Key: "gi", Description: "Go to Inbox column", Category: "Sections", Views: views},
		Keybinding{Key: "gw", Description: "Go to Waiting column", Category: "Sections", Views: views},
		Keybinding{Key: "gs", Description: "Go to Someday column", Category: "Sections", Views: views},

		Keybinding{Key: "cx", Description: "Unlink linked task", Category: "Linked Tasks", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle task selection (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "a", Description: "Select all in column (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "A", Description: "Select all across columns (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "gX", Description: "Deselect all (in multi-select)", Category: "Multi-select", Views: views},

		// Filter Popup (F prefix)
		Keybinding{Key: "C", Description: "Filter by context", Category: "Filters", Views: views},
		Keybinding{Key: "F", Description: "Open filter popup", Category: "Filters", Views: views},
		Keybinding{Key: "Fc", Description: "Toggle completed view", Category: "Filters", Views: views},
		Keybinding{Key: "Fp", Description: "Toggle high priority filter", Category: "Filters", Views: views},
		Keybinding{Key: "Ft", Description: "Filter by type (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fs", Description: "Filter by status (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "F#", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "Fd", Description: "Filter by due date (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fr", Description: "Toggle actionable/ready filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fa", Description: "Filter by attribute (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fx", Description: "Clear all filters", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Tag filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Filter summary", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Custom section manager", Category: "Filters", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Filters", Views: views},

		// Task Operations (legacy, still available)
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "o", Description: "Expand node", Category: "Task Operations", Views: views},
		Keybinding{Key: "Enter", Description: "View task details", Category: "Task Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task in editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Task Operations", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Task Operations", Views: views},
		Keybinding{Key: "cp", Description: "Cross-project linking", Category: "Task Operations", Views: views},

		// Project Views
		Keybinding{Key: "B", Description: "Project blockers", Category: "Project Views", Views: views},
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},

		// Tab Switching
		Keybinding{Key: "[/]", Description: "Cycle tabs in focused row", Category: "Navigation", Views: views},
		Keybinding{Key: "(", Description: "Switch tab row focus", Category: "Navigation", Views: views},
		Keybinding{Key: ")", Description: "Switch tab row focus", Category: "Navigation", Views: views},

		// Undo
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+Z", Description: "Undo history", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh from disk", Category: "Other", Views: views},

		// Scoring
		Keybinding{Key: "S", Description: "Score editor", Category: "Scoring", Views: views},
		Keybinding{Key: "FS", Description: "Score threshold filter", Category: "Scoring", Views: views},

		// Display
		Keybinding{Key: "O", Description: "Display settings", Category: "Display", Views: views},

		// Actions
		Keybinding{Key: "ctrl+K", Description: "Return to project tree", Category: "Navigation", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close Kanban view", Category: "Actions", Views: views},
		Keybinding{Key: "?", Description: "Show help", Category: "Actions", Views: views},
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
	r.addForViews(views, linkedTaskBindings())
	r.addForViews(views, projectCyclingBindings())
	r.addForViews(views, clipboardBindings())
}
