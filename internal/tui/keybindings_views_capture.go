package tui

func (r *KeybindingRegistry) registerQuickCaptureBindings() {
	views := []ViewType{ViewQuickCapture}
	r.add(
		Keybinding{Key: "Enter", Description: "Create task", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+O", Description: "Create task and open editor", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+V", Description: "Paste clipboard text into input", Category: "Actions", Views: views},
	)
}
