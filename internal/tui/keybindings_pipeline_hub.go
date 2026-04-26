package tui

func (r *KeybindingRegistry) registerPipelineHubBindings() {
	views := []ViewType{ViewPipelineHub}
	r.add(
		// Navigation
		Keybinding{Key: "h/l", Description: "Focus previous/next column", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Move cursor within column", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Open selected view", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}
