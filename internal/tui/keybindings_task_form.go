package tui

func (r *KeybindingRegistry) registerTaskFormBindings() {
	views := []ViewType{ViewTaskForm}
	r.add(
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab/Shift+Tab", Description: "Navigate fields", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Open picker / confirm text field", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+V", Description: "Paste clipboard text into focused field", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+E", Description: "Open description in external editor", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Open due date preset picker (Schedule tab)", Category: "Schedule", Views: views},
		Keybinding{Key: "c", Description: "Toggle checklist section", Category: "Children", Views: views},
		Keybinding{Key: "S", Description: "Toggle subtasks section", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+A", Description: "Add checklist item", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+T", Description: "Add subtask", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+D", Description: "Delete focused item", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+J", Description: "Move item down", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+K", Description: "Move item up", Category: "Children", Views: views},
		Keybinding{Key: "Ctrl+O", Description: "Toggle subtask optional", Category: "Children", Views: views},
		Keybinding{Key: "Space", Description: "Toggle checklist item done", Category: "Children", Views: views},
	)
}
