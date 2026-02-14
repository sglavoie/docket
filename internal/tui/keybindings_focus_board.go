package tui

func (r *KeybindingRegistry) registerFocusBoardBindings() {
	views := []ViewType{ViewFocusBoard}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Navigate list", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab/[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Navigate to task / open URL", Category: "Navigation", Views: views},
		Keybinding{Key: "o", Description: "Open URL in browser", Category: "Navigation", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},

		// Actions
		Keybinding{Key: "a", Description: "Quick-add board item", Category: "Actions", Views: views},
		Keybinding{Key: "x", Description: "Mark item done", Category: "Actions", Views: views},
		Keybinding{Key: "X", Description: "Dismiss item", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Toggle done/open", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete permanently", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Set priority", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit in editor", Category: "Actions", Views: views},
		Keybinding{Key: "E", Description: "Quick-edit title", Category: "Actions", Views: views},

		// Scoring
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Scoring", Views: views},

		// Filtering
		Keybinding{Key: "F", Description: "Filter (F-prefix)", Category: "Filtering", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Filtering", Views: views},
		Keybinding{Key: "Fx", Description: "Clear filters", Category: "Filtering", Views: views},

		// Sorting
		Keybinding{Key: "1-5", Description: "Sort by field", Category: "Sorting", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Sorting", Views: views},
	)
}
