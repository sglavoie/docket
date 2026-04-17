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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},

		// Filter / Search
		Keybinding{Key: "/", Description: "Search tasks (title/description)", Category: "Filter", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/due/attr/score)", Category: "Filter", Views: views},

		// View
		Keybinding{Key: "h/l or [/]", Description: "Switch tab (All/Urgent/Upcoming/Backlog/Blocked)", Category: "View", Views: views},
		Keybinding{Key: "(/) ", Description: "Toggle sub-tab row focus", Category: "View", Views: views},
		Keybinding{Key: "Tab", Description: "Toggle waiting section", Category: "View", Views: views},
		Keybinding{Key: "A", Description: "All tasks", Category: "Views", Views: views},
		Keybinding{Key: "r", Description: "Refresh (clears dismissed)", Category: "View", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close priority view", Category: "View", Views: views},
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
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close inbox processing", Category: "Navigation", Views: views},

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
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close inbox processing", Category: "Navigation", Views: views},

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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close due date overview", Category: "View", Views: views},
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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close review", Category: "View", Views: views},
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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag (tasks)", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (tasks)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Switch tab", Category: "View", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "R", Description: "Mark section reviewed", Category: "Review", Views: views},
		Keybinding{Key: "ctrl+n", Description: "Next unreviewed section", Category: "Review", Views: views},
		Keybinding{Key: "gp", Description: "Go to project/task", Category: "View", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close weekly review", Category: "View", Views: views},
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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "J", Description: "Move down in queue", Category: "Queue", Views: views},
		Keybinding{Key: "K", Description: "Move up in queue", Category: "Queue", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task / Show dismissed (Suggested)", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
		Keybinding{Key: "F", Description: "Filter prefix (type/status/tags/attr/score)", Category: "Filter", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "h/l or [/]", Description: "Navigate tabs", Category: "View", Views: views},
		Keybinding{Key: "(/) ", Description: "Toggle sub-tab row focus", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "w", Description: "Plan for today", Category: "Daily Plan", Views: views},
		Keybinding{Key: "u", Description: "Undo last plan toggle", Category: "Daily Plan", Views: views},
		Keybinding{Key: "Esc", Description: "Close view", Category: "View", Views: views},
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
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close focus view", Category: "View", Views: views},
	)
	// Only status and due-date prefixes are activated by this view's handler
	// (activateSecondaryPrefix(",dg", "", false)); priority, stage, snooze, linked
	// task prefixes are intentionally absent to avoid ghost bindings.
	r.addForViews(views, statusBindings())
	r.addForViews(views, dueDateBindings())
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
		Keybinding{Key: "o", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Sort (1=pri, 2=due, 3=new, 4=status, 5=score)", Category: "Actions", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Manage task tags", Category: "Actions", Views: views},
		Keybinding{Key: "Z", Description: "Snooze task", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Actions", Views: views},
		Keybinding{Key: "m", Description: "Move to section (ma/mn/mi/mw/ms/mr)", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+.", Description: "Quick note on selected task", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Edit score dimensions", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open URL in browser", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+b", Description: "Move task to project", Category: "Actions", Views: views},
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
		Keybinding{Key: "ctrl+e", Description: "Export task's project to Markdown", Category: "Actions", Views: views},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Queue", Views: views},
		Keybinding{Key: "Esc", Description: "Close all tasks view", Category: "View", Views: views},
	)
	r.addForViews(views, statusBindings())
	r.addForViews(views, priorityBindings())
	r.addForViews(views, stageBindings())
	r.addForViews(views, dueDateBindings())
	r.addForViews(views, snoozeBindings())
	r.addForViews(views, linkedTaskBindings())
}
