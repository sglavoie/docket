package tui

func (r *KeybindingRegistry) registerQuickCaptureBindings() {
	views := []ViewType{ViewQuickCapture}
	r.add(
		Keybinding{Key: "Enter", Description: "Create task", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+O", Description: "Create task and open editor", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+V", Description: "Paste clipboard text into input", Category: "Actions", Views: views},
	)
}

func (r *KeybindingRegistry) registerTaskCreatorBindings() {
	views := []ViewType{ViewTaskCreator}
	r.add(
		Keybinding{Key: "Tab/Shift+Tab", Description: "Navigate fields / switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Next field / confirm", Category: "Navigation", Views: views},
		Keybinding{Key: "←/→", Description: "Cycle options", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Cycle options (vim-style)", Category: "Navigation", Views: views},
		Keybinding{Key: "1-5", Description: "Direct priority selection", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Submit form", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+A", Description: "Add checklist item", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+T", Description: "Add subtask / open tag selector", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+D", Description: "Delete checklist / subtask item", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+j/k", Description: "Move item down / up", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+O", Description: "Toggle optional for subtask", Category: "Actions", Views: views},
		Keybinding{Key: "/", Description: "Open parent selector", Category: "Actions", Views: views},
		Keybinding{Key: "Space", Description: "Toggle flag / validated", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Open recurrence editor", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+V", Description: "Paste clipboard text (input) / create task from clipboard (selector)", Category: "Actions", Views: views},
	)
}
