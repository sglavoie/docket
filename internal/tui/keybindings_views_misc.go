package tui

func (r *KeybindingRegistry) registerNotificationCenterBindings() {
	views := []ViewType{ViewNotificationCenter}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate notifications", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate notifications", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "n/N", Description: "Next/previous section", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Expand/collapse notification", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Clear/restore notification", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Toggle dismissed view", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Navigate to notification", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerMessageLogBindings() {
	views := []ViewType{ViewMessageLog}
	r.add(
		Keybinding{Key: "j/k", Description: "Scroll messages", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Scroll messages", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerAskViewBindings() {
	views := []ViewType{ViewAsk}
	r.add(
		Keybinding{Key: "Ctrl+S", Description: "Submit query", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Apply extracted commands (result)", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New query (result)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerTutorBindings() {
	views := []ViewType{ViewTutor}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate list / scroll content", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate list / scroll content", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Scroll to top/bottom (content pane)", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Previous/next category", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Previous/next category", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Switch pane (list/content)", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Switch to content pane", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}
