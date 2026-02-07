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
	r.registerWeeklyReviewBindings()
	r.registerFocusViewBindings()
	r.registerProjectFocusViewBindings()
	r.registerTaskAttachmentsBindings()
	r.registerCustomContextEditorBindings()
	r.registerKanbanBindings()
	r.registerNotesBindings()
	r.registerRecentActivityBindings()
	r.registerScoreProfileManagerBindings()
	r.registerMetricsDashboardBindings()
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
		{Key: ",o", Description: "Reopen (todo)", Category: "Status"},
		{Key: ",x", Description: "Cancel task", Category: "Status"},
	}
}

func priorityBindings() []Keybinding {
	return []Keybinding{
		{Key: "!1", Description: "Set priority: Critical", Category: "⭐ Priority"},
		{Key: "!2", Description: "Set priority: High", Category: "⭐ Priority"},
		{Key: "!3", Description: "Set priority: Normal", Category: "⭐ Priority"},
		{Key: "!4", Description: "Set priority: Low", Category: "⭐ Priority"},
		{Key: "!5", Description: "Set priority: Someday", Category: "⭐ Priority"},
	}
}

func dueDateBindings() []Keybinding {
	return []Keybinding{
		{Key: "dt", Description: "Due today", Category: "⏰ Due Date"},
		{Key: "dm", Description: "Due tomorrow", Category: "⏰ Due Date"},
		{Key: "dw", Description: "Due in a week", Category: "⏰ Due Date"},
		{Key: "dn", Description: "Due in a month", Category: "⏰ Due Date"},
		{Key: "d0", Description: "Clear due date", Category: "⏰ Due Date"},
		{Key: "dd", Description: "Open date picker", Category: "⏰ Due Date"},
	}
}

func clipboardBindings() []Keybinding {
	return []Keybinding{
		{Key: "y", Description: "Copy title", Category: "📋 Clipboard"},
		{Key: "Y", Description: "Copy description", Category: "📋 Clipboard"},
		{Key: "ctrl+Y", Description: "Copy title and description", Category: "📋 Clipboard"},
	}
}

func linkedTaskBindings() []Keybinding {
	return []Keybinding{
		{Key: "cb", Description: "Create blocker", Category: "🔗 Linked Tasks"},
		{Key: "cd", Description: "Create dependent", Category: "🔗 Linked Tasks"},
		{Key: "cs", Description: "Create subtask", Category: "🔗 Linked Tasks"},
		{Key: "cf", Description: "Create follow-up", Category: "🔗 Linked Tasks"},
		{Key: "cn", Description: "Navigate linked tasks", Category: "🔗 Linked Tasks"},
		{Key: "cp", Description: "Cross-project link", Category: "🔗 Linked Tasks"},
	}
}

func projectCyclingBindings() []Keybinding {
	return []Keybinding{
		{Key: "`", Description: "Next project in cycle", Category: "Project Cycling"},
		{Key: "~", Description: "Previous project in cycle", Category: "Project Cycling"},
		{Key: "@", Description: "Configure project cycle, jump to project", Category: "Project Cycling"},
	}
}

func (r *KeybindingRegistry) registerGlobalBindings() {
	r.add(
		Keybinding{Key: ":", Description: "Command palette", Category: "Navigation"},
		Keybinding{Key: "ctrl+X", Description: "Switch project", Category: "Navigation"},
		Keybinding{Key: "alt+o", Description: "History back", Category: "Navigation"},
		Keybinding{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		Keybinding{Key: "ctrl+c", Description: "Quit", Category: "Navigation"},
		Keybinding{Key: "esc", Description: "Back / close", Category: "Navigation"},
		Keybinding{Key: "?", Description: "Toggle help", Category: "Navigation"},
	)
}
