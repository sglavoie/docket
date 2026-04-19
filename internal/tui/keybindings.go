package tui

import (
	"slices"
	"sync"
)

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
	r.registerPriorityViewBindings()
	r.registerInboxProcessorBindings()
	r.registerGlobalInboxProcessorBindings()
	r.registerDueDateOverviewBindings()
	r.registerDailyReviewBindings()
	r.registerWeeklyReviewBindings()
	r.registerFocusViewBindings()
	r.registerProjectFocusViewBindings()
	r.registerTaskAttachmentsBindings()
	r.registerCustomContextEditorBindings()
	r.registerNotesBindings()
	r.registerRecentActivityBindings()
	r.registerProjectRecentActivityBindings()
	r.registerProjectPreviewBindings()
	r.registerScoreProfileManagerBindings()
	r.registerMetricsDashboardBindings()
	r.registerAllTasksBindings()
	r.registerFocusBoardBindings()
	r.registerOutlinerBindings()
	r.registerCrossProjectOutlinerBindings()
	r.registerTaskChecklistBindings()
	r.registerTaskCommentsBindings()
	r.registerWorkflowRulesBindings()
	r.registerWorkflowDryRunBindings()
	r.registerWorkflowWizardBindings()
	r.registerTaskTemplatesBindings()
	r.registerFilterBuilderBindings()
	r.registerNotificationCenterBindings()
	r.registerMessageLogBindings()
	r.registerAskViewBindings()
	r.registerBulkEditBindings()
	r.registerBlockersBindings()
	r.registerCrossProjectLinkerBindings()
	r.registerTutorBindings()
	r.registerKeybindingExplorerBindings()
	r.registerSettingsBindings()
	r.registerRecurrenceEditorBindings()
	r.registerQuickCaptureBindings()
	r.registerTaskFormBindings()
	r.registerKanbanBindings()
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

// GetAllDeduplicated returns all keybindings with duplicates removed.
// Bindings with the same Key+Description are collapsed into one entry,
// preserving the category from the first occurrence and merging Views.
func (r *KeybindingRegistry) GetAllDeduplicated() []HelpItem {
	type dedupKey struct{ key, desc string }
	index := make(map[dedupKey]int)
	var items []HelpItem
	for _, kb := range r.bindings {
		dk := dedupKey{kb.Key, kb.Description}
		if idx, exists := index[dk]; exists {
			existing := &items[idx]
			if len(kb.Views) == 0 {
				existing.Views = nil // promote to global
			} else if existing.Views != nil {
				existing.Views = appendUniqueViews(existing.Views, kb.Views)
			}
			continue
		}
		index[dk] = len(items)
		item := HelpItem{
			Key:         kb.Key,
			Description: kb.Description,
			Category:    kb.Category,
		}
		if len(kb.Views) > 0 {
			item.Views = make([]ViewType, len(kb.Views))
			copy(item.Views, kb.Views)
		}
		items = append(items, item)
	}
	return items
}

func containsView(views []ViewType, target ViewType) bool {
	return slices.Contains(views, target)
}

func appendUniqueViews(existing, newViews []ViewType) []ViewType {
	set := make(map[ViewType]bool, len(existing))
	for _, v := range existing {
		set[v] = true
	}
	for _, v := range newViews {
		if !set[v] {
			existing = append(existing, v)
			set[v] = true
		}
	}
	return existing
}

func (r *KeybindingRegistry) add(bindings ...Keybinding) {
	r.bindings = append(r.bindings, bindings...)
}

// addForViews stamps a slice of partially-constructed bindings with the given views and adds them.
func (r *KeybindingRegistry) addForViews(views []ViewType, bindings []Keybinding) {
	for i := range bindings {
		bindings[i].Views = views
	}
	r.add(bindings...)
}

// Shared keybinding groups reused across multiple views.

func statusBindings() []Keybinding {
	return []Keybinding{
		{Key: ",s", Description: "Start (in progress)", Category: "Status"},
		{Key: ",d", Description: "Mark done", Category: "Status"},
		{Key: ",b", Description: "Block task", Category: "Status"},
		{Key: ",S", Description: "Put task on standby", Category: "Status"},
		{Key: ",o", Description: "Reopen (todo)", Category: "Status"},
		{Key: ",x", Description: "Cancel task", Category: "Status"},
		{Key: ",r", Description: "Send to review", Category: "Status"},
		{Key: ",k", Description: "Skip recurrence", Category: "Recurrence"},
		{Key: ",Dt", Description: "Defer recurrence 1 day", Category: "Recurrence"},
		{Key: ",D3", Description: "Defer recurrence 3 days", Category: "Recurrence"},
		{Key: ",Dw", Description: "Defer recurrence 1 week", Category: "Recurrence"},
		{Key: ",DW", Description: "Defer recurrence 2 weeks", Category: "Recurrence"},
		{Key: ",Dm", Description: "Defer recurrence 1 month", Category: "Recurrence"},
	}
}

func priorityBindings() []Keybinding {
	return []Keybinding{
		{Key: "!1", Description: "Set priority: Critical", Category: "Priority"},
		{Key: "!2", Description: "Set priority: High", Category: "Priority"},
		{Key: "!3", Description: "Set priority: Normal", Category: "Priority"},
		{Key: "!4", Description: "Set priority: Low", Category: "Priority"},
		{Key: "!5", Description: "Set priority: Someday", Category: "Priority"},
	}
}

func stageBindings() []Keybinding {
	return []Keybinding{
		{Key: ".1-.9", Description: "Set stage (by position)", Category: "Stage (. prefix)"},
		{Key: ".0", Description: "Clear stage", Category: "Stage (. prefix)"},
		{Key: "F.", Description: "Filter by stage", Category: "Filters"},
	}
}

func dueDateBindings() []Keybinding {
	return []Keybinding{
		{Key: "dt", Description: "Due today", Category: "Due Date"},
		{Key: "dm", Description: "Due tomorrow", Category: "Due Date"},
		{Key: "dw", Description: "Due in a week", Category: "Due Date"},
		{Key: "dn", Description: "Due in a month", Category: "Due Date"},
		{Key: "d0", Description: "Clear due date", Category: "Due Date"},
		{Key: "dd", Description: "Open date picker", Category: "Due Date"},
	}
}

func clipboardBindings() []Keybinding {
	return []Keybinding{
		{Key: "y", Description: "Copy title", Category: "Clipboard"},
		{Key: "Y", Description: "Copy description", Category: "Clipboard"},
		{Key: "ctrl+Y", Description: "Copy title and description", Category: "Clipboard"},
	}
}

func snoozeBindings() []Keybinding {
	return []Keybinding{
		{Key: "st", Description: "Snooze until tomorrow", Category: "Snooze"},
		{Key: "sw", Description: "Snooze 1 week", Category: "Snooze"},
		{Key: "sm", Description: "Snooze 1 month", Category: "Snooze"},
		{Key: "sn", Description: "Snooze until next Monday", Category: "Snooze"},
		{Key: "s0", Description: "Clear snooze", Category: "Snooze"},
		{Key: "sd", Description: "Open snooze date picker", Category: "Snooze"},
	}
}

func linkedTaskBindings() []Keybinding {
	return []Keybinding{
		{Key: "cb", Description: "Create blocker", Category: "Linked Tasks"},
		{Key: "cd", Description: "Create dependent", Category: "Linked Tasks"},
		{Key: "cs", Description: "Create subtask", Category: "Linked Tasks"},
		{Key: "cf", Description: "Create follow-up", Category: "Linked Tasks"},
		{Key: "cn", Description: "Navigate linked tasks", Category: "Linked Tasks"},
		{Key: "cp", Description: "Cross-project link", Category: "Linked Tasks"},
	}
}

func projectCyclingBindings() []Keybinding {
	return []Keybinding{
		{Key: "`", Description: "Next project in cycle", Category: "Project Cycling"},
		{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling"},
		{Key: "@", Description: "Open cycle manager", Category: "Project Cycling"},
		{Key: "@ j/k", Description: "Move cursor in popup", Category: "Project Cycling"},
		{Key: "@ space", Description: "Toggle cycle membership", Category: "Project Cycling"},
		{Key: "@ 1-9", Description: "Jump to project by number", Category: "Project Cycling"},
		{Key: "@ enter/g", Description: "Navigate to project", Category: "Project Cycling"},
		{Key: "@ p", Description: "Toggle pin on project", Category: "Project Cycling"},
		{Key: "@ c/C", Description: "Switch / clear context filter", Category: "Project Cycling"},
		{Key: "@ u", Description: "Clear all cycle selections", Category: "Project Cycling"},
		{Key: "@ ctrl+j/k", Description: "Scroll preview tree", Category: "Project Cycling"},
		{Key: "@ ctrl+h/l", Description: "Cycle preview section", Category: "Project Cycling"},
	}
}

func taskPrefixBindings() []Keybinding {
	return []Keybinding{
		{Key: "tn", Description: "New task (no template)", Category: "Task (t prefix)"},
		{Key: "tN", Description: "New task (with template picker)", Category: "Task (t prefix)"},
		{Key: "tr", Description: "Rename task (inline)", Category: "Task (t prefix)"},
		{Key: "ts", Description: "Change section", Category: "Task (t prefix)"},
		{Key: "to", Description: "Open in external editor", Category: "Task (t prefix)"},
		{Key: "tp", Description: "Pin/unpin task", Category: "Task (t prefix)"},
		{Key: "td", Description: "Dim/undim task", Category: "Task (t prefix)"},
		{Key: "tt", Description: "Manage tags", Category: "Task (t prefix)"},
		{Key: "tc", Description: "View/add comments", Category: "Task (t prefix)"},
		{Key: "ta", Description: "View/add attachments", Category: "Task (t prefix)"},
		{Key: "tl", Description: "Edit checklist", Category: "Task (t prefix)"},
		{Key: "te", Description: "Edit description", Category: "Task (t prefix)"},
		{Key: "tm", Description: "Move to another project", Category: "Task (t prefix)"},
		{Key: "tD", Description: "Delete task", Category: "Task (t prefix)"},
	}
}

func foldBindings() []Keybinding {
	return []Keybinding{
		{Key: "za", Description: "Toggle fold", Category: "Folds"},
		{Key: "zo/zc", Description: "Expand/collapse fold", Category: "Folds"},
		{Key: "zR/zM", Description: "Expand/collapse all", Category: "Folds"},
		{Key: "zv", Description: "Focus on subtree", Category: "Folds"},
		{Key: "zp", Description: "Toggle inline preview", Category: "Folds"},
	}
}

func typeChangeBindings() []Keybinding {
	return []Keybinding{
		{Key: "<", Description: "Promote type", Category: "Type Changes"},
		{Key: ">", Description: "Demote type", Category: "Type Changes"},
		{Key: "H", Description: "Cycle leaf left", Category: "Type Changes"},
		{Key: "L", Description: "Cycle leaf right", Category: "Type Changes"},
	}
}

func sortBindings() []Keybinding {
	return []Keybinding{
		{Key: "1", Description: "Sort by priority", Category: "Sort"},
		{Key: "2", Description: "Sort by due date", Category: "Sort"},
		{Key: "3", Description: "Sort by created", Category: "Sort"},
		{Key: "4", Description: "Sort by status", Category: "Sort"},
		{Key: "5", Description: "Sort by score", Category: "Sort"},
		{Key: "0", Description: "Reset sort", Category: "Sort"},
	}
}

func movePrefixBindings() []Keybinding {
	return []Keybinding{
		{Key: "ma", Description: "Send task(s) to Active", Category: "Move (m prefix)"},
		{Key: "mn", Description: "Send task(s) to Next", Category: "Move (m prefix)"},
		{Key: "mi", Description: "Send task(s) to Inbox", Category: "Move (m prefix)"},
		{Key: "mw", Description: "Send task(s) to Waiting", Category: "Move (m prefix)"},
		{Key: "mr", Description: "Send task(s) to Review", Category: "Move (m prefix)"},
		{Key: "ms", Description: "Send task(s) to Someday", Category: "Move (m prefix)"},
		{Key: "mK", Description: "Move task to top of section", Category: "Move (m prefix)"},
		{Key: "mJ", Description: "Move task to bottom of section", Category: "Move (m prefix)"},
	}
}

func blockerPrefixBindings() []Keybinding {
	return []Keybinding{
		{Key: "bb", Description: "View blockers/dependencies", Category: "Blockers (b prefix)"},
		{Key: "bm", Description: "Manage dependencies", Category: "Blockers (b prefix)"},
	}
}

func (r *KeybindingRegistry) registerGlobalBindings() {
	r.add(
		Keybinding{Key: ":", Description: "Command palette", Category: "Navigation"},
		Keybinding{Key: "|", Description: "Recent tasks", Category: "Navigation"},
		Keybinding{Key: "#", Description: "Pinned tasks", Category: "Navigation"},
		Keybinding{Key: ";n", Description: "Open Notification Center", Category: "Navigation"},
		Keybinding{Key: "ctrl+,", Description: "Open Settings", Category: "Navigation"},
		Keybinding{Key: ";d", Description: "Open Daily Review", Category: "Navigation"},
		Keybinding{Key: ";w", Description: "Open Weekly Review", Category: "Navigation"},
		Keybinding{Key: "ctrl+i", Description: "Open Global Inbox Processor", Category: "Navigation"},
		Keybinding{Key: "shift+space", Description: "Hold to peek task detail", Category: "Navigation"},
		Keybinding{Key: ";N", Description: "Open Global Notes", Category: "Navigation"},
		Keybinding{Key: ";q", Description: "Open Backlog", Category: "Navigation"},
		Keybinding{Key: ";f", Description: "Open Focus Board", Category: "Navigation"},
		Keybinding{Key: ";l", Description: "Open Recent Activity", Category: "Navigation"},
		Keybinding{Key: ";B", Description: "Open Global Blockers", Category: "Navigation"},
		Keybinding{Key: ";D", Description: "Open Due Date Overview", Category: "Navigation"},
		Keybinding{Key: ";m", Description: "Open Metrics Dashboard", Category: "Navigation"},
		Keybinding{Key: ";a", Description: "Open All Tasks", Category: "Navigation"},
		Keybinding{Key: "ctrl+/", Description: "Open Global Search", Category: "Navigation"},
		Keybinding{Key: ";W", Description: "Open Workflow Rules", Category: "Navigation"},
		Keybinding{Key: ";o", Description: "Open Global Outliner", Category: "Navigation"},
		Keybinding{Key: ";k", Description: "Open Cross-Project Kanban", Category: "Navigation"},
		Keybinding{Key: ";i", Description: "Quick Capture", Category: "Navigation"},
		Keybinding{Key: ";I", Description: "Global Bulk Capture", Category: "Navigation"},
		Keybinding{Key: ";{", Description: "Global Note Create", Category: "Navigation"},
		Keybinding{Key: ";,", Description: "Global Status Update", Category: "Navigation"},
		Keybinding{Key: ";c", Description: "Global Linked Task (create or link)", Category: "Navigation"},
		Keybinding{Key: ";tN", Description: "Global Template Task Create", Category: "Navigation"},
		Keybinding{Key: "ctrl+X", Description: "Switch project", Category: "Navigation"},
		Keybinding{Key: "ctrl+Z", Description: "Undo/redo stack", Category: "Navigation"},
		Keybinding{Key: "alt+o", Description: "History back", Category: "Navigation"},
		Keybinding{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		Keybinding{Key: "q", Description: "Toggle queue", Category: "Navigation"},
		Keybinding{Key: "ctrl+c", Description: "Quit", Category: "Navigation"},
		Keybinding{Key: "esc", Description: "Back / close", Category: "Navigation"},
		Keybinding{Key: "?", Description: "Toggle help", Category: "Navigation"},
		Keybinding{Key: `ctrl+\`, Description: "Keybinding Explorer", Category: "Navigation"},
	)
}
