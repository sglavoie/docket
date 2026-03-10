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
		Keybinding{Key: "/", Description: "Search projects", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Global tag manager", Category: "Navigation", Views: views},
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Navigation", Views: views},
		Keybinding{Key: ";W", Description: "Workflow rules", Category: "Navigation", Views: views},

		// Project Cycling
		Keybinding{Key: "`", Description: "Next project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@", Description: "Open cycle manager", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ j/k", Description: "Move cursor in popup", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ space", Description: "Toggle cycle membership", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ 1-9", Description: "Jump to project by number", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ enter/g", Description: "Navigate to project", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ p", Description: "Toggle pin on project", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ c/C", Description: "Switch / clear context filter", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ u", Description: "Clear all cycle selections", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ ctrl+j/k", Description: "Scroll preview tree", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@ ctrl+h/l", Description: "Cycle preview section", Category: "Project Cycling", Views: views},

		// Filters
		Keybinding{Key: "C", Description: "Filter by context", Category: "Filters", Views: views},

		// Actions
		Keybinding{Key: "n", Description: "New project", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit project", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Preview project", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete project", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Toggle pinned", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Toggle archived filter", Category: "Actions", Views: views},
		Keybinding{Key: "T", Description: "View trash", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Set status icon & note", Category: "Actions", Views: views},
		Keybinding{Key: "R", Description: "Cycle info panel", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh", Category: "Actions", Views: views},
		// On dashboard, "," opens Settings. In task views, "," is a status prefix key.
		// Dashboard shows projects (not tasks), so status prefix doesn't apply here.
		Keybinding{Key: ",", Description: "Settings", Category: "Actions", Views: views},
	)
}
