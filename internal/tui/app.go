package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/appearance"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewProjectCreator
	ViewProjectEditor
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewDependencyViewer
	ViewFilter
	ViewTaskJumper
	ViewProjectPreview
	ViewDueDateOverview
	ViewTagManager
	ViewGlobalTagManager
	ViewTagFilterSelector
	ViewSectionFilterSelector
	ViewSectionPicker
	ViewCustomSectionPicker
	ViewSectionManager
	ViewPriorityPicker
	ViewContextPicker
	ViewCustomContextEditor
	ViewGlobalSearch
	ViewParentSelector
	ViewPriorityView
	ViewQuickCapture
	ViewBulkCapture
	ViewInboxProcessor
	ViewTagSelector
	ViewBulkTagSelector
	ViewUndoHistory
	ViewTaskComments
	ViewCommandPalette
	ViewBulkEdit
	ViewSaveSearchInput
	ViewTrashedProjects
	ViewProjectSelector
	ViewDailyReview
	ViewWorkflowRules
	ViewWorkflowRuleCreator
	ViewWorkflowRuleEditor
	ViewWorkflowWizard
	ViewRecurrenceEditor
	ViewTextEditor
	ViewGlobalInboxProcessor
	ViewFocusView
	ViewProjectFocusView
	ViewBlockers
	ViewTaskAttachments
	ViewLinkedTaskCreator
	ViewLinkedTaskSelector
	ViewCrossProjectLinker
	ViewNotes
	ViewNoteTaskLinker
	ViewNoteTagSelector
	ViewNotesSearch
	ViewTaskActivity
	ViewRecentActivity
	ViewRecentActivityPreview
	ViewProjectRecentActivity
	ViewGlobalNotes
	ViewWeeklyReview
	ViewGlobalBlockers
	ViewSnoozeInput
	ViewTutor
	ViewAsk
	ViewProjectPicker
	ViewTaskTemplates
	ViewTaskTemplateCreator
	ViewTaskTemplateEditor
	ViewTaskTemplatePicker
	ViewTaskChecklist
	ViewTaskDetail
	ViewScoreEditor
	ViewScoreProfileManager
	ViewSaveFilterInput
	ViewMetricsDashboard
	ViewAllTasks
	ViewScoringProfilePicker
	ViewFocusBoard
	ViewOutliner
	ViewTemporaryOutliner
	ViewCrossProjectOutliner
	ViewColumnPicker
	ViewFilterBuilder
	ViewWorkflowDryRun
	ViewTemplateVarFiller
	ViewStatusInput
	ViewStatusIconPicker
	ViewNotificationCenter
	ViewMessageLog
	ViewKeybindingExplorer
	ViewProjectFilterSelector
	ViewTaskForm
	ViewKanban
	ViewCrossProjectKanban
	ViewPipelineHub
	ViewRoadmap
	ViewTaskDecisions
	ViewAttrFilterPicker
)

// ProjectTreeSpatialState captures cursor position and fold state for a project tree
// so it can survive view recreation within the same TUI session.
type ProjectTreeSpatialState struct {
	CursorTaskID string
	FoldState    map[string]bool // collapsed node IDs (Expanded == false)
}

// PriorityViewCache stores session-only filter + tab state for PriorityView.
type PriorityViewCache struct {
	Filter    CommonFilterState
	TabActive int
}

// AllTasksCache stores session-only filter + groupBy + tab state for AllTasks.
type AllTasksCache struct {
	Filter    CommonFilterState
	GroupBy   AllTasksGroupBy
	TabActive int
}

// sessionStats tracks task activity during a single TUI session for the end-of-session summary.
type sessionStats struct {
	sessionStart    time.Time
	tasksCompleted  int
	tasksCreated    int
	tasksInProgress int
	tasksSnoozed    int
}

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

	// OS appearance detector for auto-theme sync
	appearanceDetector appearance.Detector

	// Persister for project persistence operations (enables testing with mocks)
	persister Persister

	// View stack for navigation
	viewStack []ViewType
	width     int
	height    int

	// View registry for centralized view management
	views *ViewRegistry

	// Command registry for centralized command lookup (used by command palette)
	commandRegistry *CommandRegistry

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	dashboard            *DashboardModel
	projectTree          *ProjectTreeModel
	helpView             *HelpViewModel
	taskJumper           *TaskJumperModel
	globalSearch         *GlobalSearchModel
	inboxProcessor       *InboxProcessorModel
	globalInboxProcessor *GlobalInboxProcessorModel
	metricsDashboard     *MetricsDashboardModel
	roadmap              *RoadmapModel
	allTasksView         *AllTasksViewModel
	outliner             *OutlinerModel
	crossOutliner        *OutlinerModel

	// Per-project outliner toggle state (section collapse + node fold),
	// keyed by project ID. Survives outliner recreation within a session.
	outlinerToggleStates map[string]*OutlinerToggleState

	// Per-project spatial state caches (cursor + fold), keyed by project ID.
	// Survive view recreation within a session (in-memory only for cursor).
	projectTreeSpatialStates map[string]*ProjectTreeSpatialState

	// Session-only filter caches for views reconstructed during history navigation.
	priorityViewFilterCache map[string]*PriorityViewCache // keyed by projectID
	allTasksFilterCache     *AllTasksCache                // single global entry

	// Kanban view session cache — keyed by project ID or "cross_project".
	kanbanViewCache map[string]*KanbanViewCache

	// Focus view session state (persists across view navigation within a session)
	focusQueue        []FocusTask
	focusQueueIDs     map[string]bool
	focusDismissedIDs map[string]bool

	// Daily plan session state (persists across view navigation within a session)
	dailyPlan    []FocusTask
	dailyPlanIDs map[string]bool

	// openLoops caches the cross-project counts surfaced in the title row
	// (Zeigarnik). Recomputed via RefreshOpenLoopsMsg after every mutation,
	// so paint paths can read it without an extra full-tree walk.
	openLoops             OpenLoops
	dailyPlanCarried      int             // count of items carried over from a previous day
	dailyPlanCarriedIDs   map[string]bool // task IDs that were carried over (for visual indicator)
	dailyPlanCarriedShown bool            // true once the carry-over status message has been shown

	lastToggleUndoKind toggleUndoKind
	lastToggleRedoKind toggleUndoKind

	lastDailyPlanToggle     *dailyPlanToggleAction // last w-key action for undo
	lastQueueToggle         *queueToggleAction     // last q-key action for undo
	lastDailyPlanToggleRedo *dailyPlanToggleAction // last undone w-key action for redo
	lastQueueToggleRedo     *queueToggleAction     // last undone q-key action for redo

	// Last repeatable prefix-chord action for . (dot) key replay.
	lastRepeatableAction *repeatableAction

	// Current project context
	currentProject *models.Project

	// Project stack tracks project IDs for stacked ViewProjectTree entries.
	// When the project picker pushes a new project tree, the previous project ID
	// is saved so it can be restored when navigating back.
	projectStack []string

	// Terminal too narrow or short to render properly
	tooNarrow bool
	tooShort  bool

	// Session stats tracked across the lifetime of the TUI session.
	sessionStats sessionStats

	// State
	err             error
	quitting        bool
	statusMessage   string         // Temporary status message (auto-clears after timeout)
	statusIsInfo    bool           // When true, render status with InfoStatusStyle instead of SuccessStyle
	statusUndoCb    func() tea.Cmd // When non-nil and not expired, pressing `u` invokes this callback
	statusExpiresAt time.Time      // Deadline after which statusUndoCb is no longer valid

	// Today-cleared banner: shown for ~5s after the last planned Today task
	// is marked done for the first time on a given day. Takes precedence over
	// statusMessage within its TTL.
	todayClearedBanner string

	// Loading state for async operations
	isLoading      bool
	loadingMessage string
	spinner        SpinnerModel

	// Pending bulk tasks for palette operations (priority, date assignment)
	pendingBulkTasks []PaletteBulkTaskItem

	// Pending bulk section assignment from multi-select
	pendingBulkSectionTasks   []*models.Task
	pendingBulkSectionProject *models.Project

	// Pending bulk notes for multi-select operations
	pendingBulkNotes []*models.Note

	// Pending global note move operation
	pendingGlobalNoteMove *pendingNoteMove

	// Pending task creation parent (for template picker flow)
	pendingCreateTaskParent *models.Task

	// Pending template for variable filler (between picker and task creator)
	pendingVarFillerTemplate *template.TaskTemplate

	// Pending description edit context (for text editor popup flow)
	pendingDescEdit *struct {
		Task    *models.Task
		Project *models.Project
	}

	// Pending sandbox reset confirmation
	pendingSandboxReset     bool
	pendingSandboxTaskCount int

	// CLI reference for AI assistant context
	cliReference string

	// Navigation history for back/forward (Alt+O / Alt+I)
	navHistory          *NavHistory
	lastNavigatedTaskID string
	lastNavEntry        NavEntry // previous view for ctrl+6 quick toggle

	// Recent task visits for quick-switch (Alt+R)
	recentTaskVisits          []RecentTaskVisitEntry
	showQuickSwitch           bool
	quickSwitchCursor         int
	quickSwitchFilter         string
	quickSwitchFiltering      bool
	quickSwitchFiltered       []int   // indices into sortedQuickSwitchEntries
	quickSwitchTitlePositions [][]int // parallel to quickSwitchFiltered: fuzzy match positions in title
	quickSwitchPendingPrefix  string
	quickSwitchShowCompleted  bool

	// Pinned tasks popup state
	showPinnedPopup      bool
	pinnedPopupCursor    int
	pinnedPopupItems     []pinnedTaskInfo
	pinnedPendingPrefix  string
	pinnedLastUnpinned   *pinnedTaskInfo
	pinnedPopupFiltering bool
	pinnedPopupFilter    string
	pinnedPopupFiltered  []int             // indices into pinnedPopupItems
	pinnedPopupTitlePos  [][]int           // parallel to pinnedPopupFiltered: fuzzy match positions
	pinnedPreview        *ProjectTreeModel // cached preview tree for highlighted pinned task's project
	pinnedPreviewID      string            // project ID the preview was built for
	pinnedPreviewTaskID  string            // task ID the preview cursor was set for

	// Project cycle popup state
	showProjectCyclePicker bool
	showProjectCycle       bool
	projectCycleCursor     int
	projectCycleScroll     int // scroll offset for cycle popup viewport
	projectCycleItems      []ProjectInfo
	projectCyclePreview    *ProjectTreeModel // cached preview tree for highlighted project
	projectCyclePreviewID  string            // project ID the preview was built for

	// Last project-level view used (ProjectTree, Outliner) for cycle preservation
	lastProjectView ViewType

	// Per-project last view memory: remembers which project-level view was active
	// for each project so switching back restores the same view.
	projectLastView map[string]ViewType

	pendingSemicolonPrefix  bool
	pendingSemicolonTPrefix bool // true after ";t" while waiting for the sub-key (e.g. "m")

	// Proactive alert snapshots (ephemeral, regenerated on navigation)
	rawAlerts       *alerts.AlertSnapshot // unfiltered (before dismissal filter)
	proactiveAlerts *alerts.AlertSnapshot // filtered (after dismissals applied)
	dismissedSet    map[string]bool       // active dismissal keys

	// Message log: ring buffer of recent status messages for the log overlay.
	messageLog MessageLog

	// Keyboard enhancements reported by the terminal (Kitty protocol).
	keyboardEnhancements tea.KeyboardEnhancementsMsg

	// Quick note overlay (ctrl+. to add a comment without opening the full comments view)
	quickNoteOverlay *QuickNoteOverlay

	// Quick capture overlay (;i) — rendered as a floating centered box instead of a full view.
	quickCaptureOverlay *QuickCaptureModel

	// Global project picker overlay (shown for project selection in global workflows)
	globalProjectPicker *GlobalProjectPicker

	// Global task picker overlay (shown for task selection in global workflows)
	globalTaskPicker *GlobalTaskPicker

	// Status quick picker overlay (shown as step 3 of global status update flow)
	statusQuickPicker *StatusQuickPicker

	// Section quick picker overlay (shown as step 4 of global task move flow)
	sectionQuickPicker *SectionQuickPicker

	// Section change quick picker overlay (shown as step 3 of global ;ts section change flow)
	sectionChangeQuickPicker *SectionQuickPicker

	// Intermediate state for the global ;tm task-move workflow.
	// Cleared on completion or cancellation.
	globalMoveState struct {
		sourceProject *models.Project
		sourceTask    *models.Task
		destProject   *models.Project
	}

	// Link type quick picker overlay (step 3 of global ;c linked task flow)
	linkTypeQuickPicker *LinkTypeQuickPicker

	// Link mode quick picker overlay (step 4 of global ;c linked task flow)
	linkModeQuickPicker *LinkModeQuickPicker

	// Priority quick picker overlay (step 3 of global ;! priority update flow)
	priorityQuickPicker *PriorityQuickPicker

	// Due date quick picker overlay (step 3 of global ;td due date flow)
	dueDateQuickPicker *DueDateQuickPicker

	// Intermediate state for the global ;c linked task creation workflow.
	// Cleared on completion or cancellation.
	globalLinkedTaskState struct {
		sourceTask    *models.Task
		sourceProject *models.Project
		linkType      LinkType
	}

	// Peek task overlay (space to hold-peek task detail)
	peekTask        *models.Task
	peekProject     *models.Project
	peekActive      bool
	peekLeftOffset  int
	peekRightOffset int
	peekFocusRight  bool // false = left (description), true = right
	peekLeftLines   int  // cached total line count for left pane
	peekRightLines  int  // cached total line count for right pane
	peekPaneHeight  int  // cached visible height per pane

	// Export state for repeat-press clipboard behavior.
	lastExportPath string
	lastExportAt   time.Time
}

// pendingNoteMove holds state for a global note move operation.
type pendingNoteMove struct {
	Note      *models.Note
	SourceDir string
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)
