package tui

import (
	"github.com/sglavoie/tt/internal/alerts"
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
	ViewTaskCreator
	ViewTaskEditor
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
	ViewSaveViewInput
	ViewMetricsDashboard
	ViewAllTasks
	ViewScoringProfilePicker
	ViewFocusBoard
	ViewOutliner
	ViewColumnPicker
	ViewSmartListBuilder
	ViewWorkflowDryRun
	ViewTemplateVarFiller
	ViewStatusInput
	ViewStatusIconPicker
	ViewNotificationCenter
)

// ProjectTreeSpatialState captures cursor position and fold state for a project tree
// so it can survive view recreation within the same TUI session.
type ProjectTreeSpatialState struct {
	CursorTaskID string
	FoldState    map[string]bool // collapsed node IDs (Expanded == false)
}

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

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
	allTasksView         *AllTasksViewModel
	outliner             *OutlinerModel

	// Per-project outliner toggle state (section collapse + node fold),
	// keyed by project ID. Survives outliner recreation within a session.
	outlinerToggleStates map[string]*OutlinerToggleState

	// Per-project spatial state caches (cursor + fold), keyed by project ID.
	// Survive view recreation within a session (in-memory only for cursor).
	projectTreeSpatialStates map[string]*ProjectTreeSpatialState

	// Focus view session state (persists across view navigation within a session)
	focusQueue        []FocusTask
	focusQueueIDs     map[string]bool
	focusDismissedIDs map[string]bool

	// Current project context
	currentProject *models.Project

	// Project stack tracks project IDs for stacked ViewProjectTree entries.
	// When the project picker pushes a new project tree, the previous project ID
	// is saved so it can be restored when navigating back.
	projectStack []string

	// State
	err           error
	quitting      bool
	statusMessage string // Temporary status message (auto-clears after timeout)

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

	// Recent task visits for quick-switch (Alt+R)
	recentTaskVisits         []RecentTaskVisitEntry
	showQuickSwitch          bool
	quickSwitchCursor        int
	quickSwitchFilter        string
	quickSwitchFiltering     bool
	quickSwitchFiltered      []int // indices into sortedQuickSwitchEntries
	quickSwitchPendingPrefix string

	// Pinned tasks popup state
	showPinnedPopup     bool
	pinnedPopupCursor   int
	pinnedPopupItems    []pinnedTaskInfo
	pinnedPendingPrefix string
	pinnedLastUnpinned  *pinnedTaskInfo
	pinnedPreview       *ProjectTreeModel // cached preview tree for highlighted pinned task's project
	pinnedPreviewID     string            // project ID the preview was built for
	pinnedPreviewTaskID string            // task ID the preview cursor was set for

	// Project cycle popup state
	showProjectCycle      bool
	projectCycleCursor    int
	projectCycleScroll    int // scroll offset for cycle popup viewport
	projectCycleItems     []ProjectInfo
	projectCyclePreview   *ProjectTreeModel // cached preview tree for highlighted project
	projectCyclePreviewID string            // project ID the preview was built for

	// Last project-level view used (ProjectTree, Outliner) for cycle preservation
	lastProjectView ViewType

	// Per-project last view memory: remembers which project-level view was active
	// for each project so switching back restores the same view.
	projectLastView map[string]ViewType

	pendingSemicolonPrefix bool

	// Proactive alert snapshots (ephemeral, regenerated on navigation)
	rawAlerts       *alerts.AlertSnapshot // unfiltered (before dismissal filter)
	proactiveAlerts *alerts.AlertSnapshot // filtered (after dismissals applied)
	dismissedSet    map[string]bool       // active dismissal keys
}

// pendingNoteMove holds state for a global note move operation.
type pendingNoteMove struct {
	Note      *models.Note
	SourceDir string
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)
