package tui

import "sync"

// Keybinding represents a single keyboard shortcut with metadata.
type Keybinding struct {
	Key         string     // Display key string, e.g., "j/k", "ctrl+O"
	Description string     // Human-readable description
	Category    string     // Grouping for help display
	Views       []ViewType // Views where this binding is active (empty = global)
}

// KeybindingRegistry holds all keybindings for the application.
type KeybindingRegistry struct {
	bindings []Keybinding
}

var (
	keybindingRegistryOnce     sync.Once
	keybindingRegistryInstance *KeybindingRegistry
)

// GetKeybindingRegistry returns the singleton keybinding registry.
func GetKeybindingRegistry() *KeybindingRegistry {
	keybindingRegistryOnce.Do(func() {
		keybindingRegistryInstance = NewKeybindingRegistry()
	})
	return keybindingRegistryInstance
}

// NewKeybindingRegistry creates a new registry with all keybindings registered.
func NewKeybindingRegistry() *KeybindingRegistry {
	r := &KeybindingRegistry{}
	r.registerGlobalBindings()
	r.registerDashboardBindings()
	r.registerProjectTreeBindings()
	r.registerProjectCycleManagerBindings()
	r.registerPriorityViewBindings()
	r.registerInboxProcessorBindings()
	r.registerGlobalInboxProcessorBindings()
	r.registerDueDateOverviewBindings()
	r.registerDailyReviewBindings()
	r.registerFocusViewBindings()
	r.registerProjectFocusViewBindings()
	r.registerTaskAttachmentsBindings()
	r.registerCustomContextEditorBindings()
	r.registerKanbanBindings()
	r.registerNotesBindings()
	r.registerRecentActivityBindings()
	return r
}

// GetForView returns help items for the specified view, including global bindings.
func (r *KeybindingRegistry) GetForView(view ViewType) []HelpItem {
	var items []HelpItem
	for _, kb := range r.bindings {
		if len(kb.Views) == 0 || containsView(kb.Views, view) {
			items = append(items, HelpItem{
				Key:         kb.Key,
				Description: kb.Description,
				Category:    kb.Category,
			})
		}
	}
	return items
}

// GetForViewWithOverrides returns help items with custom overrides applied.
// Overrides is a map of key -> description for dynamic content.
func (r *KeybindingRegistry) GetForViewWithOverrides(view ViewType, overrides map[string]string) []HelpItem {
	items := r.GetForView(view)
	if len(overrides) == 0 {
		return items
	}
	for i, item := range items {
		if desc, ok := overrides[item.Key]; ok {
			items[i].Description = desc
		}
	}
	return items
}

// GetGlobalBindings returns bindings that apply to all views.
func (r *KeybindingRegistry) GetGlobalBindings() []HelpItem {
	var items []HelpItem
	for _, kb := range r.bindings {
		if len(kb.Views) == 0 {
			items = append(items, HelpItem{
				Key:         kb.Key,
				Description: kb.Description,
				Category:    kb.Category,
			})
		}
	}
	return items
}

func containsView(views []ViewType, target ViewType) bool {
	for _, v := range views {
		if v == target {
			return true
		}
	}
	return false
}

func (r *KeybindingRegistry) add(bindings ...Keybinding) {
	r.bindings = append(r.bindings, bindings...)
}

func (r *KeybindingRegistry) registerGlobalBindings() {
	r.add(
		Keybinding{Key: ":", Description: "Command palette", Category: "Navigation"},
		Keybinding{Key: "ctrl+c", Description: "Quit", Category: "Navigation"},
		Keybinding{Key: "esc", Description: "Back / close", Category: "Navigation"},
		Keybinding{Key: "?", Description: "Toggle help", Category: "Navigation"},
	)
}

func (r *KeybindingRegistry) registerDashboardBindings() {
	views := []ViewType{ViewDashboard}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "Select project", Category: "Navigation", Views: views},
		Keybinding{Key: "1-9", Description: "Quick select pinned", Category: "Navigation", Views: views},
		Keybinding{Key: "gg", Description: "Go to first", Category: "Navigation", Views: views},
		Keybinding{Key: "G", Description: "Go to last", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Global search", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Search projects", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+D", Description: "⏰ Due date overview", Category: "Navigation", Views: views},
		Keybinding{Key: "r", Description: "Daily review", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Global tag manager", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+W", Description: "Workflow rules", Category: "Navigation", Views: views},

		// Project Cycling
		Keybinding{Key: "`", Description: "Next project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@", Description: "Configure project cycle, jump to project", Category: "Project Cycling", Views: views},

		// Filters
		Keybinding{Key: "c", Description: "Filter by context", Category: "Filters", Views: views},

		// Views
		Keybinding{Key: "w", Description: "Global inbox processor", Category: "Views", Views: views},
		Keybinding{Key: "N", Description: "Global notes", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "🎯 Focus view", Category: "Views", Views: views},
		Keybinding{Key: "l", Description: "Latest activity", Category: "Views", Views: views},
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Views", Views: views},

		// Actions
		Keybinding{Key: "n", Description: "New project", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit project", Category: "Actions", Views: views},
		Keybinding{Key: "p", Description: "Preview project", Category: "Actions", Views: views},
		Keybinding{Key: "s", Description: "Set as current", Category: "Actions", Views: views},
		Keybinding{Key: "D", Description: "Delete project", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Toggle pinned", Category: "Actions", Views: views},
		Keybinding{Key: "a", Description: "Toggle archived filter", Category: "Actions", Views: views},
		Keybinding{Key: "T", Description: "View trash", Category: "Actions", Views: views},
		Keybinding{Key: "S", Description: "Open sandbox", Category: "Actions", Views: views},
		Keybinding{Key: "R", Description: "Toggle recent tasks", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: ",", Description: "Settings", Category: "Actions", Views: views},

		// Legend
		Keybinding{Key: "*", Description: "Current project", Category: "Legend", Views: views},
	)
}

func (r *KeybindingRegistry) registerProjectTreeBindings() {
	views := []ViewType{ViewProjectTree}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "h/l", Description: "Switch section (Active/Next/Inbox/Waiting/Someday)", Category: "Navigation", Views: views},
		Keybinding{Key: "Enter", Description: "View task (glow)", Category: "Navigation", Views: views},
		Keybinding{Key: "Space", Description: "Expand/collapse", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Cycle sidebar (hide/view/focus)", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Go to first/last", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Jump to task", Category: "Navigation", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Navigation", Views: views},
		Keybinding{Key: "f", Description: "🎯 Focus view (project)", Category: "Navigation", Views: views},

		// Folds (z prefix)
		Keybinding{Key: "za", Description: "Toggle fold", Category: "Folds", Views: views},
		Keybinding{Key: "zo/zc", Description: "Open/close fold", Category: "Folds", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all", Category: "Folds", Views: views},
		Keybinding{Key: "zv", Description: "Focus on subtree", Category: "Folds", Views: views},

		// Sidebar
		Keybinding{Key: "j/k (sidebar)", Description: "Scroll/navigate dependencies", Category: "Sidebar", Views: views},
		Keybinding{Key: "h/l (sidebar)", Description: "Switch dependency section", Category: "Sidebar", Views: views},
		Keybinding{Key: "Enter (sidebar)", Description: "Jump to dependency", Category: "Sidebar", Views: views},
		Keybinding{Key: "Esc (sidebar)", Description: "Return focus to tree", Category: "Sidebar", Views: views},

		// Status Changes (, prefix)
		Keybinding{Key: ",s", Description: "Start (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},

		// Priority Quick Actions (! prefix)
		Keybinding{Key: "!1", Description: "Set priority: Critical", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!2", Description: "Set priority: High", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!3", Description: "Set priority: Normal", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!4", Description: "Set priority: Low", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!5", Description: "Set priority: Someday", Category: "⭐ Priority", Views: views},

		// Due Date Quick Actions (d prefix)
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in a week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in a month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// Project Cycling
		Keybinding{Key: "`", Description: "Next project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@", Description: "Configure project cycle, jump to project", Category: "Project Cycling", Views: views},

		// Type Changes
		Keybinding{Key: "<", Description: "Promote type", Category: "Type Changes", Views: views},
		Keybinding{Key: ">", Description: "Demote type", Category: "Type Changes", Views: views},
		Keybinding{Key: "H", Description: "Cycle leaf left", Category: "Type Changes", Views: views},
		Keybinding{Key: "L", Description: "Cycle leaf right", Category: "Type Changes", Views: views},

		// Task Movement (when sidebar unfocused)
		Keybinding{Key: "K", Description: "Move task up", Category: "Task Movement", Views: views},
		Keybinding{Key: "J", Description: "Move task down", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Task Operations
		Keybinding{Key: "n", Description: "Add new task", Category: "Task Operations", Views: views},
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "o", Description: "Reopen task / expand node", Category: "Task Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Task Operations", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Task Operations", Views: views},
		Keybinding{Key: "r", Description: "Rename task (inline)", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+D", Description: "Set due date", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+N", Description: "Assign to section", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Pin/unpin task", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "R", Description: "View dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+B", Description: "Manage dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+G", Description: "Assign tags", Category: "Task Operations", Views: views},
		Keybinding{Key: "C", Description: "View/add comments", Category: "Task Operations", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Task Operations", Views: views},
		Keybinding{Key: "A", Description: "View/add attachments", Category: "Task Operations", Views: views},

		// Create Linked Tasks (c prefix)
		Keybinding{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cx", Description: "Unlink linked task", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "x (sidebar)", Description: "Remove link", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Task Operations", Views: views},

		// Project Views
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Project Views", Views: views},

		// Views (g prefix)
		Keybinding{Key: "gv", Description: "Cycle view (Active/Next/Inbox/Waiting/Someday/Completed)", Category: "Views", Views: views},
		Keybinding{Key: "ga", Description: "Go to Active view", Category: "Views", Views: views},
		Keybinding{Key: "gn", Description: "Go to Next view", Category: "Views", Views: views},
		Keybinding{Key: "gi", Description: "Go to Inbox view", Category: "Views", Views: views},
		Keybinding{Key: "gw", Description: "Go to Waiting view", Category: "Views", Views: views},
		Keybinding{Key: "gs", Description: "Go to Someday view", Category: "Views", Views: views},
		Keybinding{Key: "gc", Description: "Go to Completed view", Category: "Views", Views: views},
		Keybinding{Key: "go", Description: "Jump to context node's home section", Category: "Views", Views: views},
		Keybinding{Key: "gA", Description: "Send task(s) to Active", Category: "Views", Views: views},
		Keybinding{Key: "gN", Description: "Send task(s) to Next", Category: "Views", Views: views},
		Keybinding{Key: "gI", Description: "Send task(s) to Inbox", Category: "Views", Views: views},
		Keybinding{Key: "gW", Description: "Send task(s) to Waiting", Category: "Views", Views: views},
		Keybinding{Key: "gS", Description: "Send task(s) to Someday", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Inbox processor", Category: "Views", Views: views},

		// Filters
		Keybinding{Key: "/", Description: "Search tasks", Category: "Filters", Views: views},
		Keybinding{Key: "P", Description: "Filter by priority", Category: "Filters", Views: views},
		Keybinding{Key: "t", Description: "Filter by type", Category: "Filters", Views: views},
		Keybinding{Key: "S", Description: "Filter by status", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Filter by tag", Category: "Filters", Views: views},
		Keybinding{Key: "N", Description: "Filter by section", Category: "Filters", Views: views},
		Keybinding{Key: "gd", Description: "Cycle due date filter", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Filter summary", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Section manager", Category: "Filters", Views: views},

		// Sort
		Keybinding{Key: "1", Description: "Sort by priority", Category: "Sort", Views: views},
		Keybinding{Key: "2", Description: "Sort by due date", Category: "Sort", Views: views},
		Keybinding{Key: "3", Description: "Sort by created", Category: "Sort", Views: views},
		Keybinding{Key: "4", Description: "Sort by status", Category: "Sort", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Sort", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-select", Views: views},
		Keybinding{Key: "a", Description: "Select all visible (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "A", Description: "Deselect all visible (in multi-select)", Category: "Multi-select", Views: views},

		// Project
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},

		// Clipboard
		Keybinding{Key: "y", Description: "Copy title", Category: "📋 Clipboard", Views: views},
		Keybinding{Key: "Y", Description: "Copy description", Category: "📋 Clipboard", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "📋 Clipboard", Views: views},

		// Cross-Project
		Keybinding{Key: "M", Description: "Move to another project", Category: "Cross-Project", Views: views},

		// Filter Bar
		Keybinding{Key: "F", Description: "Toggle filter bar", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F1", Description: "Switch to Completed view", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F2", Description: "Toggle high priority filter", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F3", Description: "Cycle type filter", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F4", Description: "Cycle status filter", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F5", Description: "Tag filter selector", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F6", Description: "Section filter selector", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F7", Description: "Cycle due date filter", Category: "Filter Bar", Views: views},
		Keybinding{Key: "F8", Description: "Toggle actionable filter", Category: "Filter Bar", Views: views},

		// Notes
		Keybinding{Key: "[", Description: "Open notes view", Category: "Notes", Views: views},
		Keybinding{Key: "]", Description: "Create new note", Category: "Notes", Views: views},

		// Recurring Tasks
		Keybinding{Key: "n / e", Description: "Set recurrence in task creator/editor", Category: "🔁 Recurring Tasks", Views: views},
		Keybinding{Key: "i + ~", Description: "Quick capture: ~daily, ~weekly, ~2w, etc.", Category: "🔁 Recurring Tasks", Views: views},
		Keybinding{Key: ",d", Description: "Complete recurring task (creates next instance)", Category: "🔁 Recurring Tasks", Views: views},

		// Archive
		Keybinding{Key: "A", Description: "Archive completed tasks (Completed view)", Category: "📦 Archive", Views: views},

		// Other
		Keybinding{Key: "ctrl+R", Description: "Refresh", Category: "Other", Views: views},
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+Z", Description: "Undo history", Category: "Other", Views: views},
	)
}

func (r *KeybindingRegistry) registerProjectCycleManagerBindings() {
	views := []ViewType{ViewProjectCycleManager}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "/", Description: "Search projects", Category: "Navigation", Views: views},

		// Selection
		Keybinding{Key: "Space", Description: "Toggle selection", Category: "Selection", Views: views},
		Keybinding{Key: "u", Description: "Clear selection (in context)", Category: "Selection", Views: views},

		// Context
		Keybinding{Key: "c", Description: "Set context filter", Category: "Context", Views: views},
		Keybinding{Key: "C", Description: "Clear context filter", Category: "Context", Views: views},

		// Actions
		Keybinding{Key: "p", Description: "Toggle pin", Category: "Actions", Views: views},
		Keybinding{Key: "g", Description: "Go to project", Category: "Actions", Views: views},
		Keybinding{Key: "Enter", Description: "Apply selection", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Cancel", Category: "Actions", Views: views},
	)
}

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
		Keybinding{Key: "s", Description: "Start suggested task", Category: "Suggestion", Views: views},
		Keybinding{Key: "x", Description: "Dismiss suggestion", Category: "Suggestion", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "dd", Description: "Set due date", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark task done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen task (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// View
		Keybinding{Key: "Tab", Description: "Toggle waiting section", Category: "View", Views: views},
		Keybinding{Key: "r", Description: "Refresh (clears dismissed)", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close priority view", Category: "View", Views: views},

		// Create Linked Tasks (c prefix)
		Keybinding{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks", Views: views},
	)
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
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done (creates next for recurring)", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},

		// Operations
		Keybinding{Key: "s", Description: "Assign section", Category: "Operations", Views: views},
		Keybinding{Key: "p", Description: "Send to Waiting section", Category: "Operations", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (quick)", Category: "Operations", Views: views},
		Keybinding{Key: "t", Description: "Manage tags", Category: "Operations", Views: views},
		Keybinding{Key: "c", Description: "Create subtask", Category: "Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task (title/description)", Category: "Operations", Views: views},
		Keybinding{Key: "D", Description: "Set due date", Category: "Operations", Views: views},
		Keybinding{Key: "b", Description: "Manage dependencies (blockers)", Category: "Operations", Views: views},
		Keybinding{Key: "x", Description: "Delete task (with confirmation)", Category: "Operations", Views: views},
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
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},

		// Operations
		Keybinding{Key: "s", Description: "Assign section", Category: "Operations", Views: views},
		Keybinding{Key: "p", Description: "Send to Waiting section", Category: "Operations", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (quick)", Category: "Operations", Views: views},
		Keybinding{Key: "t", Description: "Manage tags", Category: "Operations", Views: views},
		Keybinding{Key: "c", Description: "Create subtask", Category: "Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task (title/description)", Category: "Operations", Views: views},
		Keybinding{Key: "D", Description: "Set due date", Category: "Operations", Views: views},
		Keybinding{Key: "b", Description: "Manage dependencies (blockers)", Category: "Operations", Views: views},
		Keybinding{Key: "x", Description: "Delete task (with confirmation)", Category: "Operations", Views: views},
		Keybinding{Key: "u", Description: "Undo last operation on current task", Category: "Operations", Views: views},
	)
}

func (r *KeybindingRegistry) registerDueDateOverviewBindings() {
	views := []ViewType{ViewDueDateOverview}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "View task details", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark task done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen task (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// Due Date
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in 1 week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in 1 month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// View
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close due date overview", Category: "View", Views: views},
	)
}

func (r *KeybindingRegistry) registerDailyReviewBindings() {
	views := []ViewType{ViewDailyReview}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},
		Keybinding{Key: "Tab", Description: "Collapse/expand section", Category: "Navigation", Views: views},
		Keybinding{Key: "n/N", Description: "Cycle sections (show all)", Category: "Navigation", Views: views},

		// Section Focus
		Keybinding{Key: "sd", Description: "Focus Overdue", Category: "Sections", Views: views},
		Keybinding{Key: "st", Description: "Focus Today", Category: "Sections", Views: views},
		Keybinding{Key: "ss", Description: "Focus Due Soon", Category: "Sections", Views: views},
		Keybinding{Key: "sl", Description: "Focus Due Later", Category: "Sections", Views: views},
		Keybinding{Key: "sf", Description: "Focus Future", Category: "Sections", Views: views},
		Keybinding{Key: "so", Description: "Focus Old", Category: "Sections", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "Jump to task in project", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark task done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen task (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// Due Date
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in 1 week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in 1 month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// View
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close review", Category: "View", Views: views},

		// Create Linked Tasks (c prefix)
		Keybinding{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks", Views: views},
	)
}

func (r *KeybindingRegistry) registerFocusViewBindings() {
	views := []ViewType{ViewFocusView}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "Jump to task in project", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Remove from focus", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark task done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen task (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// Due Date
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in 1 week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in 1 month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// View
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "gp", Description: "Go to task in project", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close focus view", Category: "View", Views: views},

		// Create Linked Tasks (c prefix)
		Keybinding{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks", Views: views},
	)
}

func (r *KeybindingRegistry) registerProjectFocusViewBindings() {
	views := []ViewType{ViewProjectFocusView}
	r.add(
		// Navigation
		Keybinding{Key: "↑/k", Description: "Move up", Category: "Navigation", Views: views},
		Keybinding{Key: "↓/j", Description: "Move down", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to top/bottom", Category: "Navigation", Views: views},
		Keybinding{Key: "ctrl+u/d", Description: "Page up/down", Category: "Navigation", Views: views},

		// Actions
		Keybinding{Key: "Enter", Description: "Close and return to tree", Category: "Actions", Views: views},
		Keybinding{Key: "*", Description: "Remove from focus", Category: "Actions", Views: views},
		Keybinding{Key: "e", Description: "Edit task", Category: "Actions", Views: views},
		Keybinding{Key: "1-5", Description: "Set priority (1=highest)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy task title", Category: "Actions", Views: views},
		Keybinding{Key: "Y", Description: "Copy task description", Category: "Actions", Views: views},

		// Status
		Keybinding{Key: ",s", Description: "Start task (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark task done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen task (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// Due Date
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in 1 week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in 1 month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// View
		Keybinding{Key: "r", Description: "Refresh", Category: "View", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close focus view", Category: "View", Views: views},
	)
}

func (r *KeybindingRegistry) registerTaskAttachmentsBindings() {
	views := []ViewType{ViewTaskAttachments}
	r.add(
		// Navigation
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "g/G", Description: "Go to first/last", Category: "Navigation", Views: views},

		// Actions
		Keybinding{Key: "a", Description: "Add attachment", Category: "Actions", Views: views},
		Keybinding{Key: "o/Enter", Description: "Open attachment", Category: "Actions", Views: views},
		Keybinding{Key: "O", Description: "Open attachments directory", Category: "Actions", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Open original path", Category: "Actions", Views: views},
		Keybinding{Key: "d/x", Description: "Delete (removes file)", Category: "Actions", Views: views},
		Keybinding{Key: "u", Description: "Unlink (keep file)", Category: "Actions", Views: views},
		Keybinding{Key: "y", Description: "Copy path to clipboard", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Actions", Views: views},
	)
}

func (r *KeybindingRegistry) registerCustomContextEditorBindings() {
	views := []ViewType{ViewCustomContextEditor}
	r.add(
		// Navigation
		Keybinding{Key: "Tab", Description: "Toggle name/projects", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k", Description: "Move cursor", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Move cursor", Category: "Navigation", Views: views},

		// Selection
		Keybinding{Key: "Space/Enter", Description: "Toggle selection", Category: "Selection", Views: views},
		Keybinding{Key: "a", Description: "Select all", Category: "Selection", Views: views},
		Keybinding{Key: "A", Description: "Deselect all", Category: "Selection", Views: views},

		// Actions
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
		Keybinding{Key: "l", Description: "Link note to task", Category: "Actions", Views: projectOnly},
		Keybinding{Key: "m", Description: "Move note to project", Category: "Actions", Views: globalOnly},
		Keybinding{Key: "p", Description: "Toggle pin", Category: "Actions", Views: views},
		Keybinding{Key: "t", Description: "Edit note tags", Category: "Tags", Views: views},
		Keybinding{Key: "T", Description: "Cycle tag filter", Category: "Tags", Views: projectOnly},
		Keybinding{Key: "/", Description: "Search notes", Category: "Filters", Views: projectOnly},
		Keybinding{Key: "Z", Description: "Toggle fullscreen pane", Category: "Navigation", Views: views},
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
		Keybinding{Key: "c", Description: "Toggle completed tasks", Category: "Actions", Views: views},
		Keybinding{Key: "r", Description: "Refresh", Category: "Actions", Views: views},
		Keybinding{Key: "q/Esc", Description: "Close", Category: "Navigation", Views: views},
	)
}

func (r *KeybindingRegistry) registerKanbanBindings() {
	views := []ViewType{ViewKanban}
	r.add(
		// Column Navigation
		Keybinding{Key: "h/l", Description: "Navigate between columns", Category: "Navigation", Views: views},

		// Task Navigation
		Keybinding{Key: "j/k", Description: "Navigate tasks within column", Category: "Navigation", Views: views},
		Keybinding{Key: "↑/↓", Description: "Navigate tasks within column", Category: "Navigation", Views: views},
		Keybinding{Key: "gg/G", Description: "Jump to first/last task in column", Category: "Navigation", Views: views},
		Keybinding{Key: "\\", Description: "Jump to task", Category: "Navigation", Views: views},

		// Split View
		Keybinding{Key: "Tab", Description: "Toggle focus between kanban/details", Category: "Navigation", Views: views},
		Keybinding{Key: "j/k (details)", Description: "Scroll description", Category: "Details Pane", Views: views},
		Keybinding{Key: "Enter (details)", Description: "View task details (glow)", Category: "Details Pane", Views: views},
		Keybinding{Key: "Esc (details)", Description: "Return to kanban", Category: "Details Pane", Views: views},

		// Task Movement Between Columns
		Keybinding{Key: "{", Description: "Move task to left column", Category: "Task Movement", Views: views},
		Keybinding{Key: "}", Description: "Move task to right column", Category: "Task Movement", Views: views},

		// Task Reordering Within Column
		Keybinding{Key: "J", Description: "Move task down in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "K", Description: "Move task up in column", Category: "Task Movement", Views: views},
		Keybinding{Key: "→", Description: "Indent (nest under above)", Category: "Task Movement", Views: views},
		Keybinding{Key: "←", Description: "Outdent (to parent level)", Category: "Task Movement", Views: views},

		// Folds (z prefix)
		Keybinding{Key: "za", Description: "Toggle fold", Category: "Folds", Views: views},
		Keybinding{Key: "zo/zc", Description: "Expand/collapse fold", Category: "Folds", Views: views},
		Keybinding{Key: "zR/zM", Description: "Expand/collapse all in column", Category: "Folds", Views: views},
		Keybinding{Key: "zv", Description: "Focus on subtree", Category: "Folds", Views: views},

		// Status Changes (, prefix)
		Keybinding{Key: ",s", Description: "Start (in progress)", Category: "Status", Views: views},
		Keybinding{Key: ",d", Description: "Mark done", Category: "Status", Views: views},
		Keybinding{Key: ",b", Description: "Block task", Category: "Status", Views: views},
		Keybinding{Key: ",o", Description: "Reopen (todo)", Category: "Status", Views: views},
		Keybinding{Key: ",x", Description: "Cancel task", Category: "Status", Views: views},

		// Priority Quick Actions (! prefix)
		Keybinding{Key: "!1", Description: "Set priority: Critical", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!2", Description: "Set priority: High", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!3", Description: "Set priority: Normal", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!4", Description: "Set priority: Low", Category: "⭐ Priority", Views: views},
		Keybinding{Key: "!5", Description: "Set priority: Someday", Category: "⭐ Priority", Views: views},

		// Due Date Quick Actions (d prefix)
		Keybinding{Key: "dt", Description: "Due today", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dw", Description: "Due in a week", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dn", Description: "Due in a month", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date", Views: views},
		Keybinding{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date", Views: views},

		// Views
		Keybinding{Key: "gc", Description: "Completed view", Category: "Views", Views: views},
		Keybinding{Key: "F1", Description: "Completed view", Category: "Views", Views: views},
		Keybinding{Key: "p", Description: "Priority view", Category: "Views", Views: views},
		Keybinding{Key: "w", Description: "Inbox processor", Category: "Views", Views: views},
		Keybinding{Key: "f", Description: "Focus on subtree (alias for zv)", Category: "Views", Views: views},

		// Section Navigation (g prefix)
		Keybinding{Key: "ga", Description: "Go to Active column", Category: "Sections", Views: views},
		Keybinding{Key: "gn", Description: "Go to Next column", Category: "Sections", Views: views},
		Keybinding{Key: "gi", Description: "Go to Inbox column", Category: "Sections", Views: views},
		Keybinding{Key: "gw", Description: "Go to Waiting column", Category: "Sections", Views: views},
		Keybinding{Key: "gs", Description: "Go to Someday column", Category: "Sections", Views: views},
		Keybinding{Key: "gA", Description: "Send task to Active", Category: "Sections", Views: views},
		Keybinding{Key: "gN", Description: "Send task to Next", Category: "Sections", Views: views},
		Keybinding{Key: "gI", Description: "Send task to Inbox", Category: "Sections", Views: views},
		Keybinding{Key: "gW", Description: "Send task to Waiting", Category: "Sections", Views: views},
		Keybinding{Key: "gS", Description: "Send task to Someday", Category: "Sections", Views: views},

		// Type Changes
		Keybinding{Key: "<", Description: "Promote type", Category: "Type Changes", Views: views},
		Keybinding{Key: ">", Description: "Demote type", Category: "Type Changes", Views: views},
		Keybinding{Key: "H", Description: "Cycle leaf left", Category: "Type Changes", Views: views},
		Keybinding{Key: "L", Description: "Cycle leaf right", Category: "Type Changes", Views: views},

		// Create Linked Tasks (c prefix)
		Keybinding{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks", Views: views},
		Keybinding{Key: "cx", Description: "Unlink linked task", Category: "🔗 Linked Tasks", Views: views},

		// Project Cycling
		Keybinding{Key: "`", Description: "Next project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling", Views: views},
		Keybinding{Key: "@", Description: "Configure project cycle, jump to project", Category: "Project Cycling", Views: views},

		// Sort
		Keybinding{Key: "1", Description: "Sort by priority", Category: "Sort", Views: views},
		Keybinding{Key: "2", Description: "Sort by due date", Category: "Sort", Views: views},
		Keybinding{Key: "3", Description: "Sort by created", Category: "Sort", Views: views},
		Keybinding{Key: "4", Description: "Sort by status", Category: "Sort", Views: views},
		Keybinding{Key: "0", Description: "Reset sort", Category: "Sort", Views: views},

		// Multi-select
		Keybinding{Key: "v", Description: "Toggle multi-select mode", Category: "Multi-select", Views: views},
		Keybinding{Key: "Space", Description: "Toggle task selection (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "a", Description: "Select all in column (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "A", Description: "Select all across columns (in multi-select)", Category: "Multi-select", Views: views},
		Keybinding{Key: "gX", Description: "Deselect all (in multi-select)", Category: "Multi-select", Views: views},

		// Filters
		Keybinding{Key: "F", Description: "Toggle filter bar", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+F", Description: "Toggle filter bar", Category: "Filters", Views: views},
		Keybinding{Key: "F2", Description: "Toggle high priority filter", Category: "Filters", Views: views},
		Keybinding{Key: "F3", Description: "Cycle type filter", Category: "Filters", Views: views},
		Keybinding{Key: "F4", Description: "Cycle status filter", Category: "Filters", Views: views},
		Keybinding{Key: "F5", Description: "Tag filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "F7", Description: "Cycle due date filter", Category: "Filters", Views: views},
		Keybinding{Key: "F8", Description: "Toggle actionable filter", Category: "Filters", Views: views},
		Keybinding{Key: "T", Description: "Tag filter selector", Category: "Filters", Views: views},
		Keybinding{Key: "ctrl+S", Description: "Section manager", Category: "Filters", Views: views},
		Keybinding{Key: "/", Description: "Search tasks", Category: "Filters", Views: views},

		// Task Operations
		Keybinding{Key: "n", Description: "Add new task", Category: "Task Operations", Views: views},
		Keybinding{Key: "i", Description: "Quick capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "I", Description: "Bulk capture", Category: "Task Operations", Views: views},
		Keybinding{Key: "o", Description: "Reopen task / expand node", Category: "Task Operations", Views: views},
		Keybinding{Key: "D", Description: "Delete task", Category: "Task Operations", Views: views},
		Keybinding{Key: "Enter", Description: "View task details (glow)", Category: "Task Operations", Views: views},
		Keybinding{Key: "e", Description: "Edit task in editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "E", Description: "Quick edit (inline with modifiers)", Category: "Task Operations", Views: views},
		Keybinding{Key: "r", Description: "Rename task (inline)", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+L", Description: "Edit last modified task", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+O", Description: "Edit in external editor", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+N", Description: "Assign to section", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+P", Description: "Pin/unpin task", Category: "Task Operations", Views: views},
		Keybinding{Key: "*", Description: "Toggle active flag", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+T", Description: "Tag manager", Category: "Task Operations", Views: views},
		Keybinding{Key: "R", Description: "View dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "ctrl+B", Description: "Manage dependencies", Category: "Task Operations", Views: views},
		Keybinding{Key: "C", Description: "View/add comments", Category: "Task Operations", Views: views},
		Keybinding{Key: "gh", Description: "View activity log", Category: "Task Operations", Views: views},
		Keybinding{Key: "A", Description: "View/add attachments", Category: "Task Operations", Views: views},
		Keybinding{Key: "M", Description: "Move to another project", Category: "Task Operations", Views: views},
		Keybinding{Key: "cp", Description: "Cross-project linking", Category: "Task Operations", Views: views},

		// Project Views
		Keybinding{Key: "b", Description: "🚧 Project blockers", Category: "Project Views", Views: views},
		Keybinding{Key: "ctrl+A", Description: "Archive/unarchive project", Category: "Project", Views: views},

		// Notes
		Keybinding{Key: "[", Description: "Open notes view", Category: "Notes", Views: views},
		Keybinding{Key: "]", Description: "Create new note", Category: "Notes", Views: views},

		// Undo
		Keybinding{Key: "u", Description: "Undo last change", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+Z", Description: "Undo history", Category: "Other", Views: views},
		Keybinding{Key: "ctrl+R", Description: "Refresh from disk", Category: "Other", Views: views},

		// Actions
		Keybinding{Key: "q/Esc", Description: "Close Kanban view", Category: "Actions", Views: views},
		Keybinding{Key: "?", Description: "Show help", Category: "Actions", Views: views},

		// Clipboard
		Keybinding{Key: "y", Description: "Copy title", Category: "📋 Clipboard", Views: views},
		Keybinding{Key: "Y", Description: "Copy description", Category: "📋 Clipboard", Views: views},
		Keybinding{Key: "ctrl+Y", Description: "Copy title and description", Category: "📋 Clipboard", Views: views},
	)
}
