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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
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

func (r *KeybindingRegistry) registerKeybindingExplorerBindings() {
	views := []ViewType{ViewKeybindingExplorer}
	r.add(
		Keybinding{Key: "/", Description: "Search keybindings", Category: "Explorer", Views: views},
		Keybinding{Key: `\`, Description: "Search and focus tab", Category: "Explorer", Views: views},
		Keybinding{Key: "Backspace", Description: "Clear search filter", Category: "Explorer", Views: views},
		Keybinding{Key: "shift+← / shift+→", Description: "Previous/next category tab", Category: "Explorer", Views: views},
		Keybinding{Key: "j/k", Description: "Scroll down/up", Category: "Explorer", Views: views},
		Keybinding{Key: "↑/↓", Description: "Scroll down/up", Category: "Explorer", Views: views},
		Keybinding{Key: "g/G", Description: "Go to top/bottom", Category: "Explorer", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Explorer", Views: views},
		Keybinding{Key: "n/N", Description: "Next/previous section", Category: "Explorer", Views: views},
		Keybinding{Key: `ctrl+\`, Description: "Close explorer", Category: "Explorer", Views: views},
		Keybinding{Key: "Esc", Description: "Clear search / close", Category: "Explorer", Views: views},
		Keybinding{Key: "?", Description: "Toggle help (no-op in explorer)", Category: "Explorer", Views: views},
	)
}

func (r *KeybindingRegistry) registerTutorBindings() {
	views := []ViewType{ViewTutor}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate list / scroll content", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate list / scroll content", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Scroll to top/bottom (content pane)", Category: "Navigation", Views: views},
		Keybinding{Key: "shift+← / shift+→", Description: "Previous/next category", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Switch pane (list/content)", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Switch to content pane", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerSettingsBindings() {
	views := []ViewType{ViewSettings}
	r.add(
		// Field navigation
		Keybinding{Key: "Tab/Shift+Tab", Description: "Navigate fields", Category: "Navigation", Views: views},
		Keybinding{Key: "shift+← / shift+→", Description: "Cycle tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "shift+↑ / shift+↓", Description: "Enter/exit sub-tab row focus", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Cycle selector left/right", Category: "Navigation", Views: views},
		Keybinding{Key: "←/→", Description: "Cycle selector left/right", Category: "Navigation", Views: views},
		// Common actions
		Keybinding{Key: "Space", Description: "Toggle boolean / cycle selector", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Save settings", Category: "Actions", Views: views},
		// Palette tab
		Keybinding{Key: "Ctrl+S", Description: "Save palette preset (Palette tab)", Category: "Palette Presets", Views: views},
		Keybinding{Key: "Ctrl+O", Description: "Open palette preset (Palette tab)", Category: "Palette Presets", Views: views},
		Keybinding{Key: "Ctrl+R", Description: "Reset palette slot to defaults (Palette tab)", Category: "Palette Presets", Views: views},
		Keybinding{Key: "D", Description: "Delete palette preset (Palette tab, load overlay)", Category: "Palette Presets", Views: views},
		// Stages tab
		Keybinding{Key: "j/k", Description: "Navigate stages (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate stages (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "a", Description: "Add stage (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "e", Description: "Edit stage (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "d/Delete", Description: "Delete stage (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "Backspace", Description: "Delete stage (Stages tab)", Category: "Stages", Views: views},
		Keybinding{Key: "J/K", Description: "Reorder stage (Stages tab)", Category: "Stages", Views: views},
		// Confirmation (stage delete, palette reset)
		Keybinding{Key: "y/Y", Description: "Confirm action", Category: "Actions", Views: views},
		Keybinding{Key: "n/N", Description: "Cancel action", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Cancel / close", Category: "Navigation", Views: views},
		Keybinding{Key: "?", Description: "Toggle help", Category: "Navigation", Views: views},
	)
}
