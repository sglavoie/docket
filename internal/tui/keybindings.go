package tui

import (
	"slices"
	"sync"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// Keybinding represents a single keyboard shortcut with metadata.
type Keybinding struct {
	Key         string     // Display key string, e.g., "j/k", "ctrl+O"
	Description string     // Human-readable description
	Category    string     // Grouping for help display
	Views       []ViewType // Views where this binding is active (empty = global)
	AppLevel    bool       // Handled by AppModel global dispatch, not view-local Update
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
	// All other views use the declarative keymap:
	viewReg := BuildRegistryFromKeyMaps(migratedViewKeymaps())
	r.add(viewReg.bindings...)
	return r
}

// BuildRegistryFromKeyMaps builds a KeybindingRegistry from a map of ViewType to KeyMap.
// Each view's non-hidden HelpItems are added as view-specific bindings.
func BuildRegistryFromKeyMaps(views map[ViewType]*keymap.KeyMap) *KeybindingRegistry {
	r := &KeybindingRegistry{}
	for view, km := range views {
		for _, item := range km.HelpItems() {
			r.add(Keybinding{
				Key:         item.Key,
				Description: item.Description,
				Category:    item.Category,
				Views:       []ViewType{view},
			})
		}
	}
	return r
}

// migratedViewKeymaps returns keymaps for all views that have been fully migrated
// to the declarative keymap system. Zero-value model instances are used here because
// buildKeyMap() only creates closures (which capture the model pointer) — the closures
// are never executed, so nil fields are safe.
func migratedViewKeymaps() map[ViewType]*keymap.KeyMap {
	return map[ViewType]*keymap.KeyMap{
		ViewDashboard:             (&DashboardModel{}).buildKeyMap(),
		ViewProjectTree:           (&ProjectTreeModel{}).buildKeyMap(),
		ViewOutliner:              (&OutlinerModel{project: &models.Project{}}).buildKeyMap(),
		ViewCrossProjectOutliner:  (&OutlinerModel{}).buildKeyMap(),
		ViewAllTasks:              (&AllTasksViewModel{}).buildKeyMap(),
		ViewFocusBoard:            (&FocusBoardModel{}).buildKeyMap(),
		ViewKanban:                (&KanbanModel{}).buildKeyMap(),
		ViewCrossProjectKanban:    (&KanbanModel{}).buildKeyMap(),
		ViewPriorityView:          (&PriorityViewModel{}).buildKeyMap(),
		ViewDueDateOverview:       (&DueDateOverviewModel{}).buildKeyMap(),
		ViewInboxProcessor:        (&InboxProcessorModel{}).buildKeyMap(),
		ViewGlobalInboxProcessor:  (&GlobalInboxProcessorModel{}).buildKeyMap(),
		ViewDailyReview:           (&DailyReviewModel{}).buildKeyMap(),
		ViewWeeklyReview:          (&WeeklyReviewModel{}).buildKeyMap(),
		ViewFocusView:             (&FocusViewModel{}).buildKeyMap(),
		ViewProjectFocusView:      (&ProjectFocusViewModel{}).buildKeyMap(),
		ViewRecentActivity:        (&RecentActivityModel{}).buildKeyMap(),
		ViewProjectRecentActivity: (&RecentActivityModel{}).buildKeyMap(),
		ViewTaskChecklist:         (&TaskChecklistModel{}).buildKeyMap(),
		ViewTaskComments:          (&TaskCommentsModel{}).buildKeyMap(),
		ViewTaskDecisions:         (&TaskDecisionsModel{}).buildKeyMap(),
		ViewWorkflowRules:         (&WorkflowRulesModel{}).buildKeyMap(),
		ViewWorkflowDryRun:        (&WorkflowDryRunModel{}).buildKeyMap(),
		ViewWorkflowWizard:        (&WorkflowWizardModel{}).buildKeyMap(),
		ViewTaskTemplates:         (&TaskTemplatesModel{}).buildKeyMap(),
		ViewFilterBuilder:         (&FilterBuilderModel{}).buildKeyMap(),
		ViewBulkEdit:              (&BulkEditModel{}).buildKeyMap(),
		ViewBlockers:              (&BlockersModel{}).buildKeyMap(),
		ViewCrossProjectLinker:    (&CrossProjectLinkerModel{}).buildKeyMap(),
		ViewNotificationCenter:    (&NotificationCenterModel{}).buildKeyMap(),
		ViewMessageLog:            (&MessageLogViewModel{}).buildKeyMap(),
		ViewAsk:                   (&AskViewModel{}).buildKeyMap(),
		ViewTutor:                 (&TutorModel{}).buildKeyMap(),
		ViewKeybindingExplorer:    (&HelpViewModel{isExplorer: true}).buildKeyMap(),
		ViewScoreProfileManager:   (&ScoreProfileManagerModel{}).buildKeyMap(),
		ViewTaskAttachments:       (&TaskAttachmentsModel{}).buildKeyMap(),
		ViewMetricsDashboard:      (&MetricsDashboardModel{}).buildKeyMap(),
		ViewRoadmap:               (&RoadmapModel{}).buildKeyMap(),
		ViewNotes:                 (&NotesViewModel{}).buildKeyMap(),
		ViewGlobalNotes:           (&NotesViewModel{}).buildKeyMap(),
		ViewPipelineHub:           (&PipelineHubModel{}).buildKeyMap(),
		ViewQuickCapture:          (&QuickCaptureModel{}).buildKeyMap(),
		ViewTaskForm:              (&TaskFormModel{}).buildKeyMap(),
		ViewProjectPreview:        (&ProjectPreviewModel{}).buildKeyMap(),
		ViewCustomContextEditor:   (&CustomContextEditorModel{}).buildKeyMap(),
		ViewSettings:              (&SettingsModel{}).buildKeyMap(),
	}
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
	return dedupHelpItems(items)
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
	return dedupHelpItems(items)
}

// GetAppLevelForView returns broad globals and scoped AppModel-level bindings
// for a view, excluding view-local keymap entries.
func (r *KeybindingRegistry) GetAppLevelForView(view ViewType) []HelpItem {
	var items []HelpItem
	for _, kb := range r.bindings {
		if len(kb.Views) == 0 || (kb.AppLevel && containsView(kb.Views, view)) {
			items = append(items, HelpItem{
				Key:         kb.Key,
				Description: kb.Description,
				Category:    kb.Category,
			})
		}
	}
	return dedupHelpItems(items)
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

func dedupHelpItems(items []HelpItem) []HelpItem {
	type dedupKey struct{ key, desc string }
	seen := make(map[dedupKey]bool, len(items))
	result := items[:0]
	for _, item := range items {
		key := dedupKey{item.Key, item.Description}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func helpItemsFromKeyMap(km *keymap.KeyMap) []HelpItem {
	raw := km.HelpItems()
	items := make([]HelpItem, len(raw))
	for i, item := range raw {
		items[i] = HelpItem{
			Key:         item.Key,
			Description: item.Description,
			Category:    item.Category,
		}
	}
	return items
}

func mergeHelpItems(groups ...[]HelpItem) []HelpItem {
	var items []HelpItem
	for _, group := range groups {
		items = append(items, group...)
	}
	return dedupHelpItems(items)
}

func (r *KeybindingRegistry) add(bindings ...Keybinding) {
	r.bindings = append(r.bindings, bindings...)
}

func scopedAppBinding(key, description, category string, views []ViewType) Keybinding {
	return Keybinding{
		Key:         key,
		Description: description,
		Category:    category,
		Views:       append([]ViewType(nil), views...),
		AppLevel:    true,
	}
}

func taskReferenceViews() []ViewType {
	return []ViewType{
		ViewProjectTree,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
		ViewAllTasks,
		ViewFocusView,
		ViewProjectFocusView,
		ViewFocusBoard,
		ViewKanban,
		ViewCrossProjectKanban,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewDueDateOverview,
		ViewPriorityView,
		ViewRecentActivity,
		ViewProjectRecentActivity,
		ViewInboxProcessor,
		ViewGlobalInboxProcessor,
		ViewTaskDetail,
	}
}

func taskPeekViews() []ViewType {
	views := taskReferenceViews()
	return slices.DeleteFunc(views, func(view ViewType) bool {
		return view == ViewTaskDetail
	})
}

func projectNotesHelpViews() []ViewType {
	return taskReferenceViews()
}

func repeatableActionViews() []ViewType {
	return []ViewType{
		ViewProjectTree,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
	}
}

func clipboardBindings(views []ViewType) []Keybinding {
	return []Keybinding{
		scopedAppBinding("y", "Copy title", "Clipboard", views),
		scopedAppBinding("Y", "Copy description", "Clipboard", views),
		scopedAppBinding("ctrl+Y", "Copy title and description", "Clipboard", views),
	}
}

func (r *KeybindingRegistry) registerGlobalBindings() {
	r.add(
		Keybinding{Key: ":", Description: "Command palette", Category: "Navigation"},
		Keybinding{Key: "|", Description: "Recent tasks", Category: "Navigation"},
		Keybinding{Key: "#", Description: "Pinned tasks", Category: "Navigation"},
		Keybinding{Key: ";;", Description: "Open Pipeline Hub", Category: "Navigation"},
		Keybinding{Key: ";n", Description: "Open Notification Center", Category: "Navigation"},
		Keybinding{Key: "ctrl+,", Description: "Open Settings", Category: "Navigation"},
		Keybinding{Key: ";d", Description: "Open Daily Review", Category: "Navigation"},
		Keybinding{Key: ";r", Description: "Open Weekly Review", Category: "Navigation"},
		Keybinding{Key: "ctrl+i", Description: "Open Global Inbox Processor", Category: "Navigation"},
		Keybinding{Key: ";N", Description: "Open Global Notes", Category: "Navigation"},
		Keybinding{Key: ";q", Description: "Open Backlog", Category: "Navigation"},
		Keybinding{Key: ";w", Description: "Open Today", Category: "Navigation"},
		Keybinding{Key: ";l", Description: "Open Recent Activity", Category: "Navigation"},
		Keybinding{Key: ";B", Description: "Open Global Blockers", Category: "Navigation"},
		Keybinding{Key: ";D", Description: "Open Due Date Overview", Category: "Navigation"},
		Keybinding{Key: ";m", Description: "Open Metrics Dashboard", Category: "Navigation"},
		Keybinding{Key: ";a", Description: "Open All Tasks", Category: "Navigation"},
		Keybinding{Key: "ctrl+/", Description: "Open Global Search", Category: "Navigation"},
		Keybinding{Key: ";W", Description: "Open Workflow Rules", Category: "Navigation"},
		Keybinding{Key: ";o", Description: "Open Global Outliner", Category: "Navigation"},
		Keybinding{Key: ";R", Description: "Open Roadmap", Category: "Navigation"},
		Keybinding{Key: ";k", Description: "Open Cross-Project Kanban", Category: "Navigation"},
		Keybinding{Key: ";i", Description: "Quick Capture", Category: "Navigation"},
		Keybinding{Key: ";I", Description: "Global Bulk Capture", Category: "Navigation"},
		Keybinding{Key: ";{", Description: "Global Note Create", Category: "Navigation"},
		Keybinding{Key: ";,", Description: "Global Status Update", Category: "Navigation"},
		Keybinding{Key: ";c", Description: "Global Linked Task (create or link)", Category: "Navigation"},
		Keybinding{Key: ";!", Description: "Global Priority Update", Category: "Navigation"},
		Keybinding{Key: ";td", Description: "Global Due Date Update", Category: "Navigation"},
		Keybinding{Key: ";tm", Description: "Global Task Move", Category: "Navigation"},
		Keybinding{Key: ";ts", Description: "Global Section Change", Category: "Navigation"},
		Keybinding{Key: ";tN", Description: "Global Template Task Create", Category: "Navigation"},
		Keybinding{Key: "ctrl+X", Description: "Switch project", Category: "Navigation"},
		Keybinding{Key: "ctrl+Z", Description: "Undo/redo stack", Category: "Navigation"},
		Keybinding{Key: "alt+o", Description: "History back", Category: "Navigation"},
		Keybinding{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		Keybinding{Key: "alt+l", Description: "Open Message Log", Category: "Navigation"},
		Keybinding{Key: "ctrl+6", Description: "Toggle last view", Category: "Navigation"},
		Keybinding{Key: "ctrl+esc", Description: "Go to Dashboard", Category: "Navigation"},
		Keybinding{Key: "ctrl+c", Description: "Quit", Category: "Navigation"},
		Keybinding{Key: "esc", Description: "Back / close", Category: "Navigation"},
		Keybinding{Key: "?", Description: "Toggle help", Category: "Navigation"},
		Keybinding{Key: `ctrl+\`, Description: "Keybinding Explorer", Category: "Navigation"},
	)
	r.add(
		scopedAppBinding("space", "Hold to peek task detail (toggles selection in multi-select)", "Navigation", taskPeekViews()),
		scopedAppBinding("q", "Toggle queue", "Navigation", taskReferenceViews()),
		scopedAppBinding("w", "Toggle today's plan", "Navigation", taskReferenceViews()),
		scopedAppBinding("ctrl+o", "Open task reference copy", "Task Operations", taskReferenceViews()),
		scopedAppBinding("ctrl+n", "Open project Notes", "Notes", projectNotesHelpViews()),
		scopedAppBinding(".", "Repeat last action", "Operations", repeatableActionViews()),
	)
	r.add(clipboardBindings(taskReferenceViews())...)
}
