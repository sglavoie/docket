package tui

func (r *KeybindingRegistry) registerTaskAttachmentsBindings() {
	views := []ViewType{ViewTaskAttachments}
	r.add(
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "a", Description: "Add attachment", Category: "Actions", Views: views},
		Keybinding{Key: "o/Enter", Description: "Open (system default)", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Open in editor", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open attachments directory", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+E", Description: "Open original path (files)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Reveal original file in file manager", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete (removes file)", Category: "Actions", Views: views},
		Keybinding{Key: "u", Description: "Unlink (keep file)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy path to clipboard", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy path with line (code refs)", Category: "Actions", Views: views},
		Keybinding{Key: "J/K", Description: "Reorder attachment", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Actions", Views: views},
	)
}

func (r *KeybindingRegistry) registerCustomContextEditorBindings() {
	views := []ViewType{ViewCustomContextEditor}
	r.add(
		Keybinding{Key: "Tab", Description: "Toggle name/projects", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search projects", Category: "Navigation", Views: views},
		Keybinding{Key: "Space/Enter", Description: "Toggle selection", Category: "Selection", Views: views},
		Keybinding{Key: "a", Description: "Select all", Category: "Selection", Views: views},
		Keybinding{Key: "A", Description: "Deselect all", Category: "Selection", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Cancel", Category: "Actions", Views: views},
	)
}

func (r *KeybindingRegistry) registerNotesBindings() {
	views := []ViewType{ViewNotes, ViewGlobalNotes}
	projectOnly := []ViewType{ViewNotes}
	globalOnly := []ViewType{ViewGlobalNotes}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "View note (glow)", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit note", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "Create new note", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Delete note (confirm)", Category: "Actions", Views: views},
		Keybinding{Key: "u", Description: "Undo last delete", Category: "Actions", Views: views},
		Keybinding{Key: "L", Description: "Link note to task", Category: "Actions", Views: projectOnly},
		Keybinding{Key: "m", Description: "Move note to project", Category: "Actions", Views: globalOnly},
		Keybinding{Key: "p", Description: "Toggle pin", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Edit note tags", Category: "Tags", Views: views},
		Keybinding{Key: "T", Description: "Cycle tag filter", Category: "Tags", Views: projectOnly},
		Keybinding{Key: "/", Description: "Search notes", Category: "Filters", Views: projectOnly},
		Keybinding{Key: "Z", Description: "Toggle fullscreen pane", Category: "Navigation", Views: views},
		Keybinding{Key: "V", Description: "Toggle multi-select mode", Category: "Multi-select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle note selection (multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "a", Description: "Select all notes (multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "s", Description: "Cycle sort mode", Category: "Filters", Views: views},
		Keybinding{Key: "r", Description: "Rename note (inline)", Category: "Actions", Views: views},
		Keybinding{Key: "C", Description: "Clear tag filters", Category: "Tags", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and body", Category: "Actions", Views: views},
		Keybinding{Key: "q", Description: "Close notes view", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerRecentActivityBindings() {
	views := []ViewType{ViewRecentActivity}
	r.add(
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Open task in project", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Preview task", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "/", Description: "Filter tasks", Category: "Filters", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/due/attr/score)", Category: "Filters", Views: views},
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerProjectRecentActivityBindings() {
	views := []ViewType{ViewProjectRecentActivity}
	r.add(
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Open task in project", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Preview task", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "/", Description: "Filter tasks", Category: "Filters", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/due/attr/score)", Category: "Filters", Views: views},
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerProjectPreviewBindings() {
	views := []ViewType{ViewProjectPreview}
	r.add(
		Keybinding{Key: "j/k", Description: "Scroll", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Scroll", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "s", Description: "Switch scoring profile", Category: "Actions", Views: views},
		Keybinding{Key: "p/q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerScoreProfileManagerBindings() {
	views := []ViewType{ViewScoreProfileManager}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "a", Description: "Add preset profile", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Delete profile", Category: "Actions", Views: views},
		Keybinding{Key: "Space/Enter", Description: "Assign to project", Category: "Actions", Views: views},
		Keybinding{Key: "u", Description: "Unassign from project", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}
