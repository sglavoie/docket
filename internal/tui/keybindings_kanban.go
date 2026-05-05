package tui

func (r *KeybindingRegistry) registerKanbanBindings() {
	views := []ViewType{ViewKanban, ViewCrossProjectKanban}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor within column", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Focus previous/next column", Category: "Navigation", Views: views},
		Keybinding{Key: "H/L", Description: "Move task to adjacent column", Category: "Navigation", Views: views},
		Keybinding{Key: "J/K", Description: "Reorder task within column", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down within column", Category: "Navigation", Views: views},
		Keybinding{Key: "G", Description: "Jump to bottom of column", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Open task detail", Category: "Navigation", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},

		// View
		Keybinding{Key: "Tab", Description: "Toggle bottom detail pane", Category: "View", Views: views},
		Keybinding{Key: "`/~", Description: "Cycle column axis mode forward/back", Category: "View", Views: views},
		Keybinding{Key: "@", Description: "Open axis mode manager", Category: "View", Views: views},
		Keybinding{Key: "?", Description: "Show help", Category: "View", Views: views},

		// Multi-select
		Keybinding{Key: "Space", Description: "Toggle multi-select on card", Category: "Multi-select", Views: views},
		Keybinding{Key: "V", Description: "Enter multi-select mode", Category: "Multi-select", Views: views},

		// Actions
		Keybinding{Key: "a", Description: "Add task (quick capture)", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze date picker", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Due date prefix (dt/dm/dw/dn/d0/dd)", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Task prefix (tn/tr/ts/td/tp/...)", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (Fp/Ft/Fs/...)", Category: "Filtering", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on focused task", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Scoring", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
		Keybinding{Key: "M", Description: "Move to custom section (picker)", Category: "Actions", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all groups", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, movePrefixBindings())
	r.addForViews(views, taskPrefixBindings())
	r.addForViews(views, blockerPrefixBindings())
}

// HandledKeys returns all keys that KanbanModel.Update() can process, for use
// by the keybinding sync test.
func (m *KanbanModel) HandledKeys() []string {
	return concatKeys(
		// Column navigation
		[]string{"j", "k", "G", "ctrl+u", "ctrl+d"},
		[]string{"h", "l"},
		// Column move and reorder
		[]string{"H", "L", "J", "K"},
		// Modal/mode keys
		[]string{"tab", "`", "~", "@", "space", "V", "?"},
		// Task navigation
		[]string{"enter", "gp", "esc"},
		// Direct action keys
		[]string{"a", "D", "e", "o", "Z", "r"},
		// Prefix activators that appear in registry
		[]string{"d", "m", "t", "F"},
		// Extra actions
		[]string{"ctrl+.", "*", "S", "O", "ctrl+b", "M"},
		// Fold keys
		[]string{"zR", "zM"},
		// Standard prefix sequences
		statusPrefixKeys(),
		priorityPrefixKeys(),
		dueDatePrefixKeys(),
		snoozePrefixKeys(),
		linkedTaskPrefixKeys(),
		stagePrefixKeys(),
		movePrefixKeys(),
		taskPrefixKeySet(),
		blockerPrefixKeySet(),
	)
}
