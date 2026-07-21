package tui

import (
	"slices"
	"sync"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// Keybinding represents a single keyboard shortcut with metadata.
type Keybinding struct {
	Key         string           // Display key string, e.g., "j/k", "ctrl+O"
	Description string           // Human-readable description
	Category    string           // Grouping for help display
	Views       []PresentationID // Views where this binding is active (empty = global)
	AppLevel    bool             // Handled by AppModel global dispatch, not view-local Update
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

// BuildRegistryFromKeyMaps builds a KeybindingRegistry from a map of PresentationID to KeyMap.
// Each view's non-hidden HelpItems are added as view-specific bindings.
func BuildRegistryFromKeyMaps(views map[PresentationID]*keymap.KeyMap) *KeybindingRegistry {
	r := &KeybindingRegistry{}
	for view, km := range views {
		for _, item := range km.HelpItems() {
			r.add(Keybinding{
				Key:         item.Key,
				Description: item.Description,
				Category:    item.Category,
				Views:       []PresentationID{view},
			})
		}
	}
	return r
}

// migratedViewKeymaps returns keymaps for all views that have been fully migrated
// to the declarative keymap system. Zero-value model instances are used here because
// buildKeyMap() only creates closures (which capture the model pointer) — the closures
// are never executed, so nil fields are safe.
func migratedViewKeymaps() map[PresentationID]*keymap.KeyMap {
	return map[PresentationID]*keymap.KeyMap{
		ViewDashboard:             (&DashboardModel{}).buildKeyMap(),
		ViewOutliner:              (&OutlinerModel{project: &models.Project{}}).buildKeyMap(),
		ViewCrossProjectOutliner:  (&OutlinerModel{}).buildKeyMap(),
		ViewAllTasks:              (&AllTasksViewModel{}).buildKeyMap(),
		ViewFocusBoard:            (&FocusBoardModel{}).buildKeyMap(),
		ViewKanban:                (&KanbanModel{}).buildKeyMap(),
		ViewCrossProjectKanban:    (&KanbanModel{}).buildKeyMap(),
		ViewPriorityView:          (&PriorityViewModel{}).buildKeyMap(),
		ViewInboxProcessor:        (&InboxProcessorModel{}).buildKeyMap(),
		ViewGlobalInboxProcessor:  (&GlobalInboxProcessorModel{}).buildKeyMap(),
		ViewDailyReview:           (&DailyReviewModel{}).buildKeyMap(),
		ViewWeeklyReview:          (&WeeklyReviewModel{}).buildKeyMap(),
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
		ViewScoreProfileManager:   (&ScoreProfileManagerModel{}).buildKeyMap(),
		ViewTaskAttachments:       (&TaskAttachmentsModel{}).buildKeyMap(),
		ViewMetricsDashboard:      (&MetricsDashboardModel{}).buildKeyMap(),
		ViewRoadmap:               (&RoadmapModel{}).buildKeyMap(),
		ViewNotes:                 (&NotesViewModel{}).buildKeyMap(),
		ViewGlobalNotes:           (&NotesViewModel{}).buildKeyMap(),
		ViewQuickCapture:          (&QuickCaptureModel{}).buildKeyMap(),
		ViewLinkedTaskCreator:     (&LinkedTaskCreatorModel{}).buildKeyMap(),
		ViewTaskForm:              (&TaskFormModel{}).buildKeyMap(),
		ViewProjectPreview:        (&ProjectPreviewModel{}).buildKeyMap(),
		ViewCustomContextEditor:   (&CustomContextEditorModel{}).buildKeyMap(),
		ViewSettings:              (&SettingsModel{}).buildKeyMap(),
		ViewConflictResolution:    (&ConflictResolutionModel{}).buildKeyMap(),
	}
}

// GetForView returns help items for the specified view, including global bindings.
func (r *KeybindingRegistry) GetForView(view PresentationID) []HelpItem {
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

// GetAppLevelForView returns broad globals and scoped AppModel-level bindings
// for a view, excluding view-local keymap entries.
func (r *KeybindingRegistry) GetAppLevelForView(view PresentationID) []HelpItem {
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
			item.Views = make([]PresentationID, len(kb.Views))
			copy(item.Views, kb.Views)
		}
		items = append(items, item)
	}
	return items
}

func containsView(views []PresentationID, target PresentationID) bool {
	return slices.Contains(views, target)
}

func appendUniqueViews(existing, newViews []PresentationID) []PresentationID {
	set := make(map[PresentationID]bool, len(existing))
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

func scopedAppBinding(key, description, category string, views []PresentationID) Keybinding {
	return Keybinding{
		Key:         key,
		Description: description,
		Category:    category,
		Views:       append([]PresentationID(nil), views...),
		AppLevel:    true,
	}
}

func taskReferenceViews() []PresentationID {
	return []PresentationID{
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
		ViewAllTasks,
		ViewProjectFocusView,
		ViewKanban,
		ViewCrossProjectKanban,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewPriorityView,
		ViewRecentActivity,
		ViewProjectRecentActivity,
		ViewInboxProcessor,
		ViewGlobalInboxProcessor,
		ViewTaskDetail,
	}
}

// queueToggleViews lists renderers where q/w planning toggles fire. Planner
// uses tab-aware creation and J/K ordering instead.
func queueToggleViews() []PresentationID {
	return taskReferenceViews()
}

func taskPeekViews() []PresentationID {
	views := append(taskReferenceViews(), ViewFocusBoard)
	return slices.DeleteFunc(views, func(view PresentationID) bool {
		return view == ViewTaskDetail
	})
}

func projectNotesHelpViews() []PresentationID {
	return taskReferenceViews()
}

func repeatableActionViews() []PresentationID {
	return []PresentationID{
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
	}
}

func clipboardBindings(views []PresentationID) []Keybinding {
	return []Keybinding{
		scopedAppBinding("y", "Copy title", "Clipboard", views),
		scopedAppBinding("Y", "Copy description", "Clipboard", views),
		scopedAppBinding("ctrl+y", "Copy title and description", "Clipboard", views),
	}
}

func (r *KeybindingRegistry) registerGlobalBindings() {
	r.add(
		Keybinding{Key: ":", Description: "Command palette", Category: "Navigation"},
		Keybinding{Key: "?", Description: "Help", Category: "Navigation"},
		Keybinding{Key: "esc", Description: "Close / back", Category: "Navigation"},
		Keybinding{Key: "ctrl+c", Description: "Quit", Category: "Navigation"},
		Keybinding{Key: "ctrl+esc", Description: "Home", Category: "Navigation"},
		Keybinding{Key: "ctrl+g", Description: "Go to destination", Category: "Navigation"},
		Keybinding{Key: "ctrl+n", Description: "Task composer", Category: "Navigation"},
		Keybinding{Key: "ctrl+x", Description: "Project picker", Category: "Navigation"},
		Keybinding{Key: "|", Description: "Recent tasks", Category: "Navigation"},
		Keybinding{Key: "#", Description: "Pinned tasks", Category: "Navigation"},
	)
	r.add(
		scopedAppBinding("enter", "Open task detail", "Navigation", taskPeekViews()),
		scopedAppBinding("shift+enter", "Peek task detail", "Navigation", taskPeekViews()),
		scopedAppBinding("q", "Toggle queue", "Navigation", queueToggleViews()),
		scopedAppBinding("w", "Toggle today's plan", "Navigation", queueToggleViews()),
		scopedAppBinding(".", "Repeat last action", "Operations", repeatableActionViews()),
	)
	r.add(clipboardBindings(taskReferenceViews())...)
}
