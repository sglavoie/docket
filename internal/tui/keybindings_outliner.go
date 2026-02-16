package tui

func (r *KeybindingRegistry) registerOutlinerBindings() {
	views := []ViewType{ViewOutliner}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+d/u", Description: "Page down/up", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Zoom into parent / open leaf", Category: "Navigation", Views: views},
		Keybinding{Key: "Backspace", Description: "Zoom out one level", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Zoom out / close", Category: "Navigation", Views: views},
		Keybinding{Key: "1-9", Description: "Switch perspective", Category: "Navigation", Views: views},

		// View switching
		Keybinding{Key: "ctrl+P", Description: "Project tree view", Category: "Views", Views: views},
		Keybinding{Key: "ctrl+K", Description: "Kanban board view", Category: "Views", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Focus board view", Category: "Views", Views: views},

		// Editing
		Keybinding{Key: "o", Description: "Rename task (inline)", Category: "Editing", Views: views},
		Keybinding{Key: "O/E", Description: "Quick edit with modifiers", Category: "Editing", Views: views},
		Keybinding{Key: "e", Description: "Open in external editor", Category: "Editing", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Editing", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-Select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle selection", Category: "Multi-Select", Views: views},
		Keybinding{Key: "a/A", Description: "Select all / deselect all", Category: "Multi-Select", Views: views},

		// Scoring
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Scoring", Views: views},
	)

	// Shared bindings
	r.addForViews(views, statusBindings())
	r.addForViews(views, clipboardBindings())

	// Outliner sort uses s-prefix (s1-s5, s0)
	r.add(
		Keybinding{Key: "s1", Description: "Sort by priority", Category: "Sort (s prefix)", Views: views},
		Keybinding{Key: "s2", Description: "Sort by status", Category: "Sort (s prefix)", Views: views},
		Keybinding{Key: "s3", Description: "Sort by due date", Category: "Sort (s prefix)", Views: views},
		Keybinding{Key: "s4", Description: "Sort by created", Category: "Sort (s prefix)", Views: views},
		Keybinding{Key: "s5", Description: "Sort by score", Category: "Sort (s prefix)", Views: views},
		Keybinding{Key: "s0", Description: "Reset sort", Category: "Sort (s prefix)", Views: views},
	)

	// Outliner task prefix bindings
	r.add(
		Keybinding{Key: "tn", Description: "New subtask", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "tN", Description: "New sibling task", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "to", Description: "Edit description", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "tc", Description: "View/add comments", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "ta", Description: "View/add attachments", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "tl", Description: "Navigate linked tasks", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "td", Description: "Manage dependencies", Category: "Task (t prefix)", Views: views},
		Keybinding{Key: "tm", Description: "Move to another project", Category: "Task (t prefix)", Views: views},
	)
}
