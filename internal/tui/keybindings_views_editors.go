package tui

func (r *KeybindingRegistry) registerTaskChecklistBindings() {
	views := []ViewType{ViewTaskChecklist}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "Space/Enter", Description: "Toggle done/undone", Category: "Actions", Views: views},
		Keybinding{Key: "a/n", Description: "Add item", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit item", Category: "Actions", Views: views},
		Keybinding{Key: "E", Description: "Edit in external editor", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete item", Category: "Actions", Views: views},
		Keybinding{Key: "J/K", Description: "Reorder item", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save (add/edit mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+E", Description: "Open in external editor (add mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerTaskCommentsBindings() {
	views := []ViewType{ViewTaskComments}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "a/n", Description: "Add comment", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit comment", Category: "Actions", Views: views},
		Keybinding{Key: "E", Description: "Edit in external editor", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Toggle resolved", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete comment", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy comment text", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Expand comment", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save (add/edit mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+E", Description: "Open in external editor (add/edit mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerWorkflowRulesBindings() {
	views := []ViewType{ViewWorkflowRules}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search rules", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle enabled/disabled", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New rule (form)", Category: "Actions", Views: views},
		Keybinding{Key: "w", Description: "New rule (wizard)", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit rule", Category: "Actions", Views: views},
		Keybinding{Key: "d/D", Description: "Delete rule", Category: "Actions", Views: views},
		Keybinding{Key: "J/K", Description: "Adjust rule priority", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Dry run all rules", Category: "Actions", Views: views},
		Keybinding{Key: "T", Description: "Test selected rule", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Preview (narrow screen)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerWorkflowDryRunBindings() {
	views := []ViewType{ViewWorkflowDryRun}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerWorkflowWizardBindings() {
	views := []ViewType{ViewWorkflowWizard}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate list", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate list", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Select / confirm", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Back one step / cancel", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerTaskTemplatesBindings() {
	views := []ViewType{ViewTaskTemplates}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search templates", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+D/U", Description: "Scroll preview", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle enabled/disabled", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New template", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit template", Category: "Actions", Views: views},
		Keybinding{Key: "d/D", Description: "Delete template", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Preview (narrow screen)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerFilterBuilderBindings() {
	views := []ViewType{ViewFilterBuilder}
	r.add(
		Keybinding{Key: "[/]", Description: "Switch tabs (Criteria/Options)", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab/Shift+Tab", Description: "Navigate fields", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Navigate fields (vim-style)", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate fields", Category: "Navigation", Views: views},
		Keybinding{Key: "←/→", Description: "Cycle field/op/value", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Cycle field/op/value (vim-style)", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Next field", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle (logic/sort/completed)", Category: "Actions", Views: views},
		Keybinding{Key: "+/-", Description: "Add/remove criterion", Category: "Criteria", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save smart list", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Cancel", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerBulkEditBindings() {
	views := []ViewType{ViewBulkEdit}
	r.add(
		Keybinding{Key: "Tab/j", Description: "Next field", Category: "Navigation", Views: views},
		Keybinding{Key: "↓", Description: "Next field", Category: "Navigation", Views: views},
		Keybinding{Key: "Shift+Tab/k", Description: "Previous field", Category: "Navigation", Views: views},
		Keybinding{Key: "↑", Description: "Previous field", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Cycle option left/right", Category: "Navigation", Views: views},
		Keybinding{Key: "←/→", Description: "Cycle option left/right", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle selection", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Confirm", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Add tag", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Remove tag", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Cancel", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerBlockersBindings() {
	views := []ViewType{ViewBlockers}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "a/n", Description: "Add blocker", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit blocker", Category: "Actions", Views: views},
		Keybinding{Key: "E", Description: "Edit in external editor", Category: "Actions", Views: views},
		Keybinding{Key: "R", Description: "Toggle resolved", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete blocker", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Expand blocker", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save (edit/add mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+E", Description: "Open in external editor (add/edit mode)", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerCrossProjectLinkerBindings() {
	views := []ViewType{ViewCrossProjectLinker}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Select", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Back / Close", Category: "Navigation", Views: views},
	)
}
