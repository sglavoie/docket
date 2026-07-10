package tui

// Common prefix hints used across multiple views.
var (
	// StatusPrefixHints contains hints for the "," (status) prefix.
	StatusPrefixHints = []PrefixHint{
		{Key: "s", Description: "start", Detail: "Set task to in-progress"},
		{Key: "d", Description: "done", Detail: "Mark task as completed"},
		{Key: "b", Description: "block", Detail: "Mark task as blocked"},
		{Key: "x", Description: "cancel", Detail: "Cancel the task", Danger: true},
		{Key: "o", Description: "reopen", Detail: "Reopen a closed task"},
		{Key: "r", Description: "review", Detail: "Send task to review"},
		{Key: "S", Description: "standby", Detail: "Put task on standby"},
		{Key: "m", Description: "maybe", Detail: "Set task to maybe"},
		{Key: "k", Description: "skip", Detail: "Skip next recurrence"},
		{Key: "D", Description: "defer", Detail: "Defer recurrence"},
	}

	// FoldPrefixHints contains hints for the "f" (fold) prefix.
	FoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
		{Key: "p", Description: "preview", Detail: "Toggle inline preview for cursor task"},
		{Key: "1", Description: "depth-1", Detail: "Collapse to top level only"},
		{Key: "2", Description: "depth-2", Detail: "Expand to second level"},
		{Key: "3", Description: "depth-3", Detail: "Expand to third level"},
	}

	// NavigationPrefixHints contains hints for the "g" (go/navigation) prefix - basic version.
	NavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
	}

	// PriorityViewNavigationPrefixHints contains "g" prefix hints for priority view.
	PriorityViewNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
		{Key: "t", Description: "tree", Detail: "Switch to tree view"},
	}

	// ProjectTreeNavigationPrefixHints contains extended "g" prefix hints for project tree.
	ProjectTreeNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first", Detail: "Jump to first item"},
		{Key: "a", Description: "active", Detail: "Go to Active section"},
		{Key: "n", Description: "next", Detail: "Go to Next section"},
		{Key: "i", Description: "inbox", Detail: "Go to Inbox section"},
		{Key: "w", Description: "waiting", Detail: "Go to Waiting section"},
		{Key: "s", Description: "someday", Detail: "Go to Someday section"},
		{Key: "c", Description: "completed", Detail: "Go to Completed section"},
		{Key: "r", Description: "review", Detail: "Go to Review section"},
		{Key: "S", Description: "standby", Detail: "Go to Standby section"},
		{Key: "z", Description: "snoozed", Detail: "Go to Snoozed section"},
		{Key: "*", Description: "all", Detail: "Go to All section"},
		{Key: "o", Description: "origin", Detail: "Jump to task origin"},
		{Key: "d", Description: "due-filter", Detail: "Toggle due date filter"},
		{Key: "v", Description: "view-cycle", Detail: "Cycle through view modes"},
		{Key: "h", Description: "history", Detail: "Show task history"},
		{Key: "X", Description: "deselect", Detail: "Clear selection"},
		{Key: "l", Description: "activity", Detail: "Show project recent activity"},
		{Key: "k", Description: "kanban", Detail: "Open kanban view for current project"},
	}

	// ProjectTreeMovePrefixHints contains "m" prefix hints for project tree move operations.
	ProjectTreeMovePrefixHints = []PrefixHint{
		{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
		{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
		{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
		{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
		{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
		{Key: "S", Description: "→Standby", Detail: "Send task to Standby section"},
		{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
		{Key: "K", Description: "to-top", Detail: "Move task to top of section"},
		{Key: "J", Description: "to-bottom", Detail: "Move task to bottom of section"},
	}

	// PriorityPrefixHintsBase contains hints for the "!" (priority) prefix
	// without icons or active state. Use BuildPriorityPrefixHints for overlays.
	PriorityPrefixHintsBase = []PrefixHint{
		{Key: "1", Description: "critical", Detail: "Set priority to critical"},
		{Key: "2", Description: "high", Detail: "Set priority to high"},
		{Key: "3", Description: "normal", Detail: "Set priority to normal"},
		{Key: "4", Description: "low", Detail: "Set priority to low"},
		{Key: "5", Description: "someday", Detail: "Set priority to someday"},
	}

	// DueDatePrefixHints contains hints for the "d" (due date) prefix.
	DueDatePrefixHints = []PrefixHint{
		{Key: "t", Description: "today", Detail: "Set due date to today"},
		{Key: "m", Description: "tomorrow", Detail: "Set due date to tomorrow"},
		{Key: "w", Description: "+week", Detail: "Set due date one week out"},
		{Key: "n", Description: "+month", Detail: "Set due date one month out"},
		{Key: "0", Description: "clear", Detail: "Remove due date"},
		{Key: "d", Description: "picker", Detail: "Open date picker"},
	}

	// SnoozePrefixHints contains hints for the "z" (snooze) prefix.
	SnoozePrefixHints = []PrefixHint{
		{Key: "t", Description: "tomorrow", Detail: "Snooze until tomorrow"},
		{Key: "w", Description: "+week", Detail: "Snooze for one week"},
		{Key: "m", Description: "+month", Detail: "Snooze for one month"},
		{Key: "n", Description: "next Mon", Detail: "Snooze until next Monday"},
		{Key: "0", Description: "clear", Detail: "Remove snooze date"},
		{Key: "d", Description: "picker", Detail: "Open date picker"},
	}

	// CreateLinkedPrefixHints contains hints for the "c" (create linked) prefix.
	CreateLinkedPrefixHints = []PrefixHint{
		{Key: "b", Description: "blocker", Detail: "Create a blocking task"},
		{Key: "d", Description: "dependent", Detail: "Create a dependent task"},
		{Key: "s", Description: "subtask", Detail: "Create a subtask"},
		{Key: "f", Description: "follow-up", Detail: "Create a follow-up task"},
		{Key: "p", Description: "cross-project", Detail: "Link to another project"},
		{Key: "n", Description: "navigate", Detail: "Navigate to linked task"},
		{Key: "x", Description: "unlink", Detail: "Remove a link"},
	}

	// OutlinerFoldPrefixHints contains "f" prefix hints for outliner fold operations.
	OutlinerFoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle", Detail: "Toggle fold on current node"},
		{Key: "s", Description: "section", Detail: "Toggle fold on current section"},
		{Key: "S", Description: "sections", Detail: "Toggle fold on all visible sections"},
		{Key: "v", Description: "focus", Detail: "Focus on current subtree"},
		{Key: "p", Description: "preview", Detail: "Toggle inline preview for cursor task"},
		{Key: "1", Description: "depth-1", Detail: "Collapse to top level only"},
		{Key: "2", Description: "depth-2", Detail: "Expand to second level"},
		{Key: "3", Description: "depth-3", Detail: "Expand to third level"},
	}

	// TaskPrefixHints contains hints for the "t" (task action) prefix.
	TaskPrefixHints = []PrefixHint{
		{Key: "n", Description: "new", Detail: "Create new task"},
		{Key: "N", Description: "new-template", Detail: "Create task with template picker"},
		{Key: "r", Description: "rename", Detail: "Rename task (inline)"},
		{Key: "s", Description: "section", Detail: "Change section"},
		{Key: "o", Description: "open", Detail: "Open in external editor"},
		{Key: "p", Description: "pin", Detail: "Toggle pin/unpin"},
		{Key: "P", Description: "park", Detail: "Note where you left off"},
		{Key: "h", Description: "hide", Detail: "Dim/undim task"},
		{Key: "t", Description: "tags", Detail: "Manage task tags"},
		{Key: "c", Description: "comments", Detail: "View/add comments"},
		{Key: "a", Description: "attachments", Detail: "View/add attachments"},
		{Key: "l", Description: "checklist", Detail: "Edit checklist items"},
		{Key: "d", Description: "decisions", Detail: "View/add decisions"},
		{Key: "e", Description: "description", Detail: "Edit description"},
		{Key: "m", Description: "move", Detail: "Move to another project"},
		{Key: "D", Description: "delete", Detail: "Delete task", Danger: true},
	}

	// SemicolonPrefixHints contains hints for the ";" (tt global) prefix.
	SemicolonPrefixHints = []PrefixHint{
		{Key: ";", Description: "pipeline hub", Detail: "Overview of all pipeline stages", Group: ""},
		{Key: "!", Description: "priority", Detail: "Change task priority globally", Group: "Support"},
		{Key: ",", Description: "status update", Detail: "Change task status globally", Group: "Support"},
		{Key: "a", Description: "all tasks", Detail: "View all tasks across projects", Group: "Triage"},
		{Key: "c", Description: "linked task", Detail: "Create or link a related task", Group: "Support"},
		{Key: "d", Description: "daily review", Detail: "Open Daily Review", Group: "Plan"},
		{Key: "w", Description: "today", Detail: "Today cockpit", Group: "Work"},
		{Key: "i", Description: "quick capture", Detail: "Full-width task capture view", Group: "Capture"},
		{Key: "l", Description: "latest activity", Detail: "Recently modified tasks", Group: "Review"},
		{Key: "m", Description: "metrics", Detail: "Completion rates and throughput", Group: "Review"},
		{Key: "n", Description: "notifications", Detail: "Open Notification Center", Group: "Support"},
		{Key: "k", Description: "kanban", Detail: "Cross-project kanban", Group: "Triage"},
		{Key: "o", Description: "outliner", Detail: "Cross-project outliner", Group: "Triage"},
		{Key: "q", Description: "backlog", Detail: "Cross-project task suggestions", Group: "Plan"},
		{Key: "t", Description: "task...", Detail: "Task actions", Group: "Support"},
		{Key: "r", Description: "weekly review", Detail: "Open Weekly Review", Group: "Review"},
		{Key: "B", Description: "global blockers", Detail: "Blockers across all projects", Group: "Support"},
		{Key: "D", Description: "due date overview", Detail: "View tasks by due date", Group: "Plan"},
		{Key: "I", Description: "bulk capture", Detail: "Bulk task capture in a project", Group: "Capture"},
		{Key: "N", Description: "global notes", Detail: "Open global notes", Group: "Support"},
		{Key: "R", Description: "roadmap", Detail: "Gantt chart and critical path", Group: "Review"},
		{Key: "W", Description: "workflow rules", Detail: "Manage workflow automation rules", Group: "Support"},
		{Key: "{", Description: "note create", Detail: "Create note in a project", Group: "Support"},
	}

	// SemicolonTPrefixHints contains hints for the ";t" (task actions) sub-prefix.
	SemicolonTPrefixHints = []PrefixHint{
		{Key: "d", Description: "due date", Detail: "Set task due date globally"},
		{Key: "m", Description: "move", Detail: "Move task to another project"},
		{Key: "s", Description: "section", Detail: "Change task section globally"},
		{Key: "N", Description: "template", Detail: "Create task from template"},
	}

	// BlockerPrefixHints contains hints for the "b" (blocker/dependency) prefix.
	BlockerPrefixHints = []PrefixHint{
		{Key: "b", Description: "view", Detail: "View blockers/dependencies"},
		{Key: "m", Description: "manage", Detail: "Manage dependencies"},
	}
)
