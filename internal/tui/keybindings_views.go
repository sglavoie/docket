package tui

func (r *KeybindingRegistry) registerPriorityViewBindings() {
	views := []ViewType{ViewPriorityView}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "gt", Description: "Go to task in tree", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},

		// Suggestion
		Keybinding{Key: "w", Description: "Start suggested task", Category: "Suggestion", Views: views},
		Keybinding{Key: "x", Description: "Dismiss suggestion", Category: "Suggestion", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},

		// Filter / Search
		Keybinding{Key: "/", Description: "Search tasks (title/description)", Category: "Filter", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/due/attr/score)", Category: "Filter", Views: views},

		// View
		Keybinding{Key: "h/l or [/]", Description: "Switch tab (All/Urgent/Upcoming/Backlog/Blocked)", Category: "View", Views: views},
		Keybinding{Key: "(/) ", Description: "Toggle sub-tab row focus", Category: "View", Views: views},
		Keybinding{Key: "Tab", Description: "Toggle waiting section", Category: "View", Views: views},
		Keybinding{Key: "A", Description: "All tasks", Category: "Views", Views: views},
		Keybinding{Key: "r", Description: "Refresh (clears dismissed)", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close priority view", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

func (r *KeybindingRegistry) registerInboxProcessorBindings() {
	views := []ViewType{ViewInboxProcessor}
	r.add(
		// Navigation
		Keybinding{Key: "enter", Description: "View task details", Category: "Navigation", Views: views},
		Keybinding{Key: "n", Description: "Skip task (move to next without processing)", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Navigate to next/previous task", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Go to first/last task", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Quit inbox processing", Category: "Navigation", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done (creates next for recurring)", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},

		// Operations
		Keybinding{Key: "s", Description: "Assign section", Category: "Operations", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (quick)", Category: "Operations", Views: views},
		Keybinding{Key: "t", Description: "Manage tags", Category: "Operations", Views: views},
		Keybinding{Key: "c", Description: "Create subtask", Category: "Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task (title/description)", Category: "Operations", Views: views},
		Keybinding{Key: "d", Description: "Set due date", Category: "Operations", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Operations", Views: views},
		Keybinding{Key: "b", Description: "Manage dependencies (blockers)", Category: "Operations", Views: views},
		Keybinding{Key: "w", Description: "Send to Waiting section", Category: "Operations", Views: views},
		Keybinding{Key: "u", Description: "Undo last operation on current task", Category: "Operations", Views: views},

		// Recurring Tasks
		Keybinding{Key: "e", Description: "Edit task to set/view recurrence pattern", Category: "Recurring", Views: views},
	)
}

func (r *KeybindingRegistry) registerGlobalInboxProcessorBindings() {
	views := []ViewType{ViewGlobalInboxProcessor}
	r.add(
		// Navigation
		Keybinding{Key: "enter", Description: "View task details", Category: "Navigation", Views: views},
		Keybinding{Key: "n", Description: "Skip task (move to next without processing)", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Navigate to next/previous task", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Go to first/last task", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Quit inbox processing", Category: "Navigation", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},

		// Operations
		Keybinding{Key: "s", Description: "Assign section", Category: "Operations", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (quick)", Category: "Operations", Views: views},
		Keybinding{Key: "t", Description: "Manage tags", Category: "Operations", Views: views},
		Keybinding{Key: "c", Description: "Create subtask", Category: "Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task (title/description)", Category: "Operations", Views: views},
		Keybinding{Key: "d", Description: "Set due date", Category: "Operations", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Operations", Views: views},
		Keybinding{Key: "b", Description: "Manage dependencies (blockers)", Category: "Operations", Views: views},
		Keybinding{Key: "w", Description: "Send to Waiting section", Category: "Operations", Views: views},
		Keybinding{Key: "u", Description: "Undo last operation on current task", Category: "Operations", Views: views},
	)
}

func (r *KeybindingRegistry) registerDueDateOverviewBindings() {
	views := []ViewType{ViewDueDateOverview}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close due date overview", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

func (r *KeybindingRegistry) registerDailyReviewBindings() {
	views := []ViewType{ViewDailyReview}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Collapse/expand section", Category: "Navigation", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all sections", Category: "Navigation", Views: views},
		Keybinding{Key: "zp", Description: "Toggle inline preview", Category: "Navigation", Views: views},
		Keybinding{Key: "n/N", Description: "Cycle sections (show all)", Category: "Navigation", Views: views},
		Keybinding{Key: "gd", Description: "Focus Overdue", Category: "Sections", Views: views},
		Keybinding{Key: "gt", Description: "Focus Today", Category: "Sections", Views: views},
		Keybinding{Key: "gs", Description: "Focus Due Soon", Category: "Sections", Views: views},
		Keybinding{Key: "gl", Description: "Focus Due Later", Category: "Sections", Views: views},
		Keybinding{Key: "gf", Description: "Focus Future", Category: "Sections", Views: views},
		Keybinding{Key: "gr", Description: "Focus In Review", Category: "Sections", Views: views},
		Keybinding{Key: "go", Description: "Focus Old", Category: "Sections", Views: views},
		Keybinding{Key: "Enter", Description: "Jump to task in project", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close review", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

func (r *KeybindingRegistry) registerWeeklyReviewBindings() {
	views := []ViewType{ViewWeeklyReview}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Collapse/expand section", Category: "Navigation", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all sections", Category: "Navigation", Views: views},
		Keybinding{Key: "zp", Description: "Toggle inline preview", Category: "Navigation", Views: views},
		Keybinding{Key: "n/N", Description: "Cycle sections (show all)", Category: "Navigation", Views: views},
		Keybinding{Key: "gP", Description: "Focus Stale Projects", Category: "Sections", Views: views},
		Keybinding{Key: "gi", Description: "Focus Inbox Backlog", Category: "Sections", Views: views},
		Keybinding{Key: "gb", Description: "Focus Blocked Work", Category: "Sections", Views: views},
		Keybinding{Key: "go", Description: "Focus Overcommitted", Category: "Sections", Views: views},
		Keybinding{Key: "gt", Description: "Focus Stale Tasks", Category: "Sections", Views: views},
		Keybinding{Key: "gm", Description: "Focus Priority Mismatch", Category: "Sections", Views: views},
		Keybinding{Key: "gw", Description: "Focus Upcoming Week", Category: "Sections", Views: views},
		Keybinding{Key: "Enter", Description: "Jump to project/task", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag (tasks)", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (tasks)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "R", Description: "Mark section reviewed", Category: "Review", Views: views},
		Keybinding{Key: "ctrl+n", Description: "Next unreviewed section", Category: "Review", Views: views},
		Keybinding{Key: "gp", Description: "Go to project/task", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close weekly review", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

func (r *KeybindingRegistry) registerFocusViewBindings() {
	views := []ViewType{ViewFocusView}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Jump to task in project", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Accept into queue", Category: "Queue", Views: views},
		Keybinding{Key: "x", Description: "Dismiss / Remove from queue", Category: "Queue", Views: views},
		Keybinding{Key: "J", Description: "Move down in queue", Category: "Queue", Views: views},
		Keybinding{Key: "K", Description: "Move up in queue", Category: "Queue", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task / Show dismissed (Suggested)", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Navigate tabs", Category: "View", Views: views},
		Keybinding{Key: "(/) ", Description: "Toggle sub-tab row focus", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close view", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, linkedTaskBindings())
}

func (r *KeybindingRegistry) registerProjectFocusViewBindings() {
	views := []ViewType{ViewProjectFocusView}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Close and return to tree", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Remove from focus", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "(/) ", Description: "Toggle sub-tab row focus", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Navigate sub-tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close focus view", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

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
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Actions", Views: views},
	)
}

func (r *KeybindingRegistry) registerCustomContextEditorBindings() {
	views := []ViewType{ViewCustomContextEditor}
	r.add(
		Keybinding{Key: "Tab", Description: "Toggle name/projects", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
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
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "s", Description: "Switch scoring profile", Category: "Actions", Views: views},
		Keybinding{Key: "p/q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerScoreProfileManagerBindings() {
	views := []ViewType{ViewScoreProfileManager}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "a", Description: "Add preset profile", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Delete profile", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Assign to project", Category: "Actions", Views: views},
		Keybinding{Key: "u", Description: "Unassign from project", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerAllTasksBindings() {
	views := []ViewType{ViewAllTasks}
	r.add(
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project tree", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Sort (1=pri, 2=due, 3=new, 4=status, 5=score)", Category: "Actions", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "v", Description: "Toggle multi-select", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/due/attr/score/project)", Category: "Filter", Views: views},
		Keybinding{Key: "FP", Description: "Toggle project filter", Category: "Filter", Views: views},
		Keybinding{Key: "n/N", Description: "Next/prev group header", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+g", Description: "Cycle grouping mode", Category: "View", Views: views},
		Keybinding{Key: "Tab", Description: "Collapse/expand group", Category: "View", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all groups", Category: "View", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Navigate tabs", Category: "View", Views: views},
		Keybinding{Key: "{/}", Description: "Move tab", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "+", Description: "Create smart list tab", Category: "Smart Lists", Views: views},
		Keybinding{Key: "E", Description: "Edit current smart list tab", Category: "Smart Lists", Views: views},
		Keybinding{Key: "X", Description: "Delete current smart list tab", Category: "Smart Lists", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close all tasks view", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}

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
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
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
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerWorkflowRulesBindings() {
	views := []ViewType{ViewWorkflowRules}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search rules", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle enabled/disabled", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New rule", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit rule", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Delete rule", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Dry run all rules", Category: "Actions", Views: views},
		Keybinding{Key: "T", Description: "Test selected rule", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Preview (narrow screen)", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerWorkflowDryRunBindings() {
	views := []ViewType{ViewWorkflowDryRun}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Navigation", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerTaskTemplatesBindings() {
	views := []ViewType{ViewTaskTemplates}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Top/Bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search templates", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+D/U", Description: "Scroll preview", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle enabled/disabled", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New template", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit template", Category: "Actions", Views: views},
		Keybinding{Key: "d", Description: "Delete template", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Preview (narrow screen)", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerFilterBuilderBindings() {
	views := []ViewType{ViewFilterBuilder}
	r.add(
		Keybinding{Key: "[/]", Description: "Switch tabs (Criteria/Options)", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab/Shift+Tab", Description: "Navigate fields", Category: "Navigation", Views: views},
		Keybinding{Key: "←/→", Description: "Cycle field/op/value", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Next field", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle (logic/sort/completed)", Category: "Actions", Views: views},
		Keybinding{Key: "+/-", Description: "Add/remove criterion", Category: "Criteria", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save smart list", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Cancel", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerNotificationCenterBindings() {
	views := []ViewType{ViewNotificationCenter}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate notifications", Category: "Navigation", Views: views},
		Keybinding{Key: "d/x", Description: "Clear notification", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Clear all notifications", Category: "Actions", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerMessageLogBindings() {
	views := []ViewType{ViewMessageLog}
	r.add(
		Keybinding{Key: "j/k", Description: "Scroll messages", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerAskViewBindings() {
	views := []ViewType{ViewAsk}
	r.add(
		Keybinding{Key: "Ctrl+S", Description: "Submit query", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Apply extracted commands (result)", Category: "Actions", Views: views},
		Keybinding{Key: "n", Description: "New query (result)", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerBulkEditBindings() {
	views := []ViewType{ViewBulkEdit}
	r.add(
		Keybinding{Key: "Tab/j", Description: "Next field", Category: "Navigation", Views: views},
		Keybinding{Key: "Shift+Tab/k", Description: "Previous field", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Cycle option left/right", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Toggle selection", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Confirm", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Add tag", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Remove tag", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Cancel", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerBlockersBindings() {
	views := []ViewType{ViewBlockers}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Switch tabs", Category: "Navigation", Views: views},
		Keybinding{Key: "a/n", Description: "Add blocker", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit blocker", Category: "Actions", Views: views},
		Keybinding{Key: "E", Description: "Edit in external editor", Category: "Actions", Views: views},
		Keybinding{Key: "R", Description: "Toggle resolved", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete blocker", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Expand blocker", Category: "Actions", Views: views},
		Keybinding{Key: "Ctrl+S", Description: "Save (edit/add mode)", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerCrossProjectLinkerBindings() {
	views := []ViewType{ViewCrossProjectLinker}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Select", Category: "Actions", Views: views},
		Keybinding{Key: "Esc", Description: "Back / Close", Category: "Navigation", Views: views},
		Keybinding{Key: "q", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerTutorBindings() {
	views := []ViewType{ViewTutor}
	r.add(
		Keybinding{Key: "j/k", Description: "Navigate list / scroll content", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Previous/next category", Category: "Navigation", Views: views},
		Keybinding{Key: "[/]", Description: "Previous/next category", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Switch pane (list/content)", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search", Category: "Navigation", Views: views},
		Keybinding{Key: "Ctrl+U/D", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}
