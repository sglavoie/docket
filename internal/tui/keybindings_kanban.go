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
		Keybinding{Key: "Enter (details)", Description: "View task details (glow)", Category: "Details Pane", Views: views},
		Keybinding{Key: "Esc (details)", Description: "Return to kanban", Category: "Details Pane", Views: views},

		// Task Movement Between Columns
		Keybinding{Key: "{", Description: "Move task to left column", Category: "Task Movement", Views: views},
		Keybinding{Key: "}", Description: "Move task to right column", Category: "Task Movement", Views: views},

		// Task Reordering Within Column
		Keybinding{Key: "J", Description: "Move task down in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "K", Description: "Move task up in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Folds (z prefix)
		Keybinding{Key: "za", Description: "Toggle fold", Category: "Folds", Views: views},
		Keybinding{Key: "zo/zc", Description: "Expand/collapse fold", Category: "Folds", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all in column", Category: "Folds", Views: views},
		Keybinding{Key: "zv", Description: "Focus on subtree", Category: "Folds", Views: views},

		// Views
		Keybinding{Key: "gc", Description: "Completed view", Category: "Views", Views: views},
		Keybinding{Key: "g*", Description: "All view (all sections in one list)", Category: "Views", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Inbox processor", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "Focus on subtree (alias for zv)", Category: "Views", Views: views},

		// Section Navigation (g prefix)
		Keybinding{Key: "ga", Description: "Go to Active column", Category: "Sections", Views: views},
		Keybinding{Key: "gn", Description: "Go to Next column", Category: "Sections", Views: views},
		Keybinding{Key: "gi", Description: "Go to Inbox column", Category: "Sections", Views: views},
		Keybinding{Key: "gw", Description: "Go to Waiting column", Category: "Sections", Views: views},
		Keybinding{Key: "gs", Description: "Go to Someday column", Category: "Sections", Views: views},
		Keybinding{Key: "gA", Description: "Send task to Active", Category: "Sections", Views: views},
		Keybinding{Key: "gN", Description: "Send task to Next", Category: "Sections", Views: views},
		Keybinding{Key: "gI", Description: "Send task to Inbox", Category: "Sections", Views: views},
		Keybinding{Key: "gW", Description: "Send task to Waiting", Category: "Sections", Views: views},
		Keybinding{Key: "gS", Description: "Send task to Someday", Category: "Sections", Views: views},

		// Type Changes
		Keybinding{Key: "<", Description: "Promote type", Category: "Type Changes", Views: views},
		Keybinding{Key: ">", Description: "Demote type", Category: "Type Changes", Views: views},
		Keybinding{Key: "H", Description: "Cycle leaf left", Category: "Type Changes", Views: views},
		Keybinding{Key: "L", Description: "Cycle leaf right", Category: "Type Changes", Views: views},

		Keybinding{Key: "cx", Description: "Unlink linked task", Category: "🔗 Linked Tasks", Views: views},

		// Sort
		Keybinding{Key: "1", Description: "Sort by priority", Category: "Sort", Views: views},
		Keybinding{Key: "2", Description: "Sort by due date", Category: "Sort", Views: views},
		Keybinding{Key: "3", Description: "Sort by created", Category: "Sort", Views: views},
		Keybinding{Key: "4", Description: "Sort by status", Category: "Sort", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Sort", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle task selection (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "a", Description: "Select all in column (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "A", Description: "Select all across columns (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "gX", Description: "Deselect all (in multi-select)", Category: "Multi-select", Views: views},

		// Filter Popup (F prefix)
		Keybinding{Key: "F", Description: "Open filter popup", Category: "Filters", Views: views},
		Keybinding{Key: "Fc", Description: "Toggle completed view", Category: "Filters", Views: views},
		Keybinding{Key: "Fp", Description: "Toggle high priority filter", Category: "Filters", Views: views},
		Keybinding{Key: "Ft", Description: "Filter by type (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fs", Description: "Filter by status (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "F#", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "Fd", Description: "Filter by due date (sub-popup)", Category: "Filters", Views: views},
		Keybinding{Key: "Fr", Description: "Toggle actionable/ready filter", Category: "Filters", Views: views},
		Keybinding{Key: "Fx", Description: "Clear all filters", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Tag filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Custom section manager", Category: "Filters", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Filters", Views: views},

		// Task Operations
		Keybinding{Key: "n", Description: "Add new task", Category: "Task Operations", Views: views},
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "o", Description: "Reopen task / expand node", Category: "Task Operations", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Task Operations", Views: views},
		Keybinding{Key: "Enter", Description: "View task details (glow)", Category: "Task Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task in editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Task Operations", Views: views},
		Keybinding{Key: "r", Description: "Rename task (inline)", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+N", Description: "Assign to section", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Pin/unpin task", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+G", Description: "Assign tags (multi-select)", Category: "Task Operations", Views: views},
		Keybinding{Key: "R", Description: "View dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+B", Description: "Manage dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "C", Description: "View/add comments", Category: "Task Operations", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Task Operations", Views: views},
		Keybinding{Key: "A", Description: "View/add attachments", Category: "Task Operations", Views: views},
		Keybinding{Key: "M", Description: "Move to another project", Category: "Task Operations", Views: views},
		Keybinding{Key: "cp", Description: "Cross-project linking", Category: "Task Operations", Views: views},

		// Project Views
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Project Views", Views: views},
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},

		// Tab Switching
		Keybinding{Key: "[/]", Description: "Cycle tabs in focused row", Category: "Navigation", Views: views},
		Keybinding{Key: "(", Description: "Switch tab row focus", Category: "Navigation", Views: views},
		Keybinding{Key: ")", Description: "Switch tab row focus", Category: "Navigation", Views: views},

		// Undo
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+Z", Description: "Undo history", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh from disk", Category: "Other", Views: views},

		// Actions
		Keybinding{Key: "q/Esc", Description: "Close Kanban view", Category: "Actions", Views: views},
		Keybinding{Key: "?", Description: "Show help", Category: "Actions", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, linkedTaskBindings())
	r.addForViews(views, projectCyclingBindings())
	r.addForViews(views, clipboardBindings())
}
