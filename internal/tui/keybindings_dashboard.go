package tui

func (r *KeybindingRegistry) registerDashboardBindings() {
	views := []ViewType{ViewDashboard}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Select project", Category: "Navigation", Views: views},
		Keybinding{Key: "1-9", Description: "Quick select pinned", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Global search", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Search projects", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+D", Description: "⏰ Due date overview", Category: "Navigation", Views: views},
		Keybinding{Key: "r", Description: "Daily review", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Global tag manager", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+W", Description: "Workflow rules", Category: "Navigation", Views: views},

		// Project Cycling
		Keybinding{Key: "`", Description: "Next project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@", Description: "Configure project cycle, jump to project", Category: "Project Cycling", Views: views},

		// Filters
		Keybinding{Key: "C", Description: "Filter by context", Category: "Filters", Views: views},

		// Views
		Keybinding{Key: "w", Description: "Global inbox processor", Category: "Views", Views: views},
		Keybinding{Key: "N", Description: "Global notes", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "🎯 Focus view", Category: "Views", Views: views},
		Keybinding{Key: "l", Description: "Latest activity", Category: "Views", Views: views},
		Keybinding{Key: "W", Description: "Weekly review", Category: "Views", Views: views},
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Views", Views: views},
		Keybinding{Key: "B", Description: "Global project blockers", Category: "Views", Views: views},

		// Actions
		Keybinding{Key: "n", Description: "New project", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit project", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Preview project", Category: "Actions", Views: views},
		Keybinding{Key: "s", Description: "Set as current", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete project", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Toggle pinned", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Toggle archived filter", Category: "Actions", Views: views},
		Keybinding{Key: "T", Description: "View trash", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Open sandbox", Category: "Actions", Views: views},
		Keybinding{Key: "R", Description: "Toggle info panel", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: ",", Description: "Settings", Category: "Actions", Views: views},

		// Legend
		Keybinding{Key: "*", Description: "Current project", Category: "Legend", Views: views},
	)
}
