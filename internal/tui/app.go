package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/appearance"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// PresentationID represents the different views in the TUI.
type PresentationID int

const (
	ViewDashboard PresentationID = iota
	ViewProjectList
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
	ViewTagManager
	ViewGlobalTagManager
	ViewTagFilterSelector
	ViewSectionFilterSelector
	ViewSectionPicker
	ViewSectionManager
	ViewPriorityPicker
	ViewGlobalSearch
	ViewParentSelector
	ViewQuickCapture
	ViewBulkCapture
	ViewInboxProcessor
	ViewTagSelector
	ViewBulkTagSelector
	ViewTaskComments
	ViewCommandPalette
	ViewBulkEdit
	ViewSaveSearchInput
	ViewProjectSelector
	ViewDailyReview
	ViewWorkflowRules
	ViewWorkflowRuleCreator
	ViewWorkflowRuleEditor
	ViewWorkflowWizard
	ViewRecurrenceEditor
	ViewTextEditor
	ViewGlobalInboxProcessor
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
	ViewParkInput
	ViewTutor
	ViewAsk
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
	ViewScoringProfilePicker
	ViewFocusBoard
	ViewOutliner
	ViewCrossProjectOutliner
	ViewColumnPicker
	ViewFilterBuilder
	ViewWorkflowDryRun
	ViewTemplateVarFiller
	ViewStatusInput
	ViewStatusIconPicker
	ViewNotificationCenter
	ViewMessageLog
	ViewTaskForm
	ViewRoadmap
	ViewTaskDecisions
	ViewAttrFilterPicker
	ViewConflictResolution
	ViewContextHub
)

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
	configManager *config.Manager

	// runtime is the startup-loaded snapshot of the five configuration domains
	// (catalog, planning, settings, automation, machine), their load-time raw
	// revisions, and the workspace data dir. It is the composition-root state the
	// shell holds as the sole configuration state:
	// views read domain data through its narrow accessors without render-time disk
	// loads, config-bearing mutations commit only the changed domain(s) through its
	// revision-checked CommitCatalog/CommitPlanning/CommitSettings/CommitAutomation/
	// CommitMachine seams, and each commit refreshes the snapshot so the views
	// observe the committed state without a further disk load.
	runtime *config.RuntimeState

	// deps is the domain-native dependency bundle (RuntimeState + application
	// service + config manager + workspace ledger) shared by the shell and
	// feature groups.
	deps *shellDeps

	// projects preserves the loaded models.Project pointers used by open views.
	// RuntimeState deliberately rehydrates fresh catalog models on every read;
	// this shell-local cache keeps task trees stable while RuntimeState remains
	// the only owner of persisted catalog metadata.
	projects map[string]*models.Project

	// OS appearance detector for auto-theme sync
	appearanceDetector appearance.Detector

	// Persister for project persistence operations (enables testing with mocks)
	persister Persister

	// features are the application-backed coordinators the primary feature views
	// dispatch their mutations and queries through. They route persistence
	// through the application layer (one atomic ChangeSet per mutation) instead of
	// letting a view mutate and save its own model copy. See feature_coordinator.go.
	features FeatureCoordinators

	// secondaryFeatures are the application-backed coordinators the secondary
	// views and overlays (bulk operations, attachments, tags, blockers, settings,
	// workflow/scoring editors) dispatch their mutations through. Each depends on
	// a narrow host port satisfied by the same application service, so an overlay
	// routes persistence through one atomic ChangeSet (or config write) instead of
	// a direct saveProjectSync / configManager.Save call. See
	// feature_coordinator_secondary.go.
	secondaryFeatures SecondaryCoordinators

	// reviewHub is the sole live Review destination. Daily, Weekly, Activity,
	// Metrics, and Roadmap remain calculation/rendering modules owned by it.
	reviewHub *ReviewHubModel

	// manageHub is the sole live Manage destination for scoped collections,
	// notifications, and the message log. Session 12 extends the same shell with
	// automation, templates, scoring, and settings.
	manageHub *ManageHubModel

	// taskDetail is the sole live task-scoped primary child. Checklist, comments,
	// decisions, attachments, and activity are embedded tabs rather than sibling
	// routes; PresentationID identifies the active tab renderer.
	taskDetail *TaskDetailModel

	// taskComposer is the sole live compact/expanded task mutation overlay.
	// Capture/form presentation IDs remain observations for embedded modes.
	taskComposer *TaskComposerModel

	// appService is the same application service the feature coordinators wrap,
	// retained directly for the few AppModel-owned overlays that mutate
	// synchronously in Update() and need the typed result inline (the pinned-task
	// popup: unpin/repin/reorder). Routing through it keeps those mutations atomic
	// and ledger-undoable without a direct raw save.
	appService *app.Service

	// navigation is the live typed shell state. viewStack mirrors implementation
	// renderer identity for registries and headless tests; production dispatch
	// resolves through navigation.
	navigation      NavigationState
	typedNavigation bool
	viewStack       []PresentationID
	startupMessage  tea.Msg
	startupRoute    *Route
	startupView     PresentationID
	width           int
	height          int

	// View registry for centralized view management
	views *ViewRegistry

	// Command registry for centralized command lookup (used by command palette)
	commandRegistry *CommandRegistry

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	helpView             *HelpViewModel
	taskJumper           *TaskJumperModel
	inboxProcessor       *InboxProcessorModel
	globalInboxProcessor *GlobalInboxProcessorModel
	metricsDashboard     *MetricsDashboardModel
	roadmap              *RoadmapModel
	outliner             *OutlinerModel
	crossOutliner        *OutlinerModel

	// Per-project outliner toggle state (section collapse + node fold),
	// keyed by project ID. Survives outliner recreation within a session.
	outlinerToggleStates map[string]*OutlinerToggleState

	// Focus view session state (persists across view navigation within a session)
	focusQueue        []FocusTask
	focusQueueIDs     map[TaskIdentity]bool
	focusDismissedIDs map[string]bool

	// Daily plan session state (persists across view navigation within a session)
	dailyPlan    []FocusTask
	dailyPlanIDs map[TaskIdentity]bool

	// openLoops caches the cross-project counts surfaced in the title row
	// (Zeigarnik). Recomputed via RefreshOpenLoopsMsg after every mutation,
	// so paint paths can read it without an extra full-tree walk.
	openLoops             OpenLoops
	dailyPlanCarried      int                   // count of items carried over from a previous day
	dailyPlanCarriedIDs   map[TaskIdentity]bool // tasks that were carried over (for visual indicator)
	dailyPlanCarriedShown bool                  // true once the carry-over status message has been shown

	lastToggleUndoKind toggleUndoKind
	lastToggleRedoKind toggleUndoKind

	lastDailyPlanToggle     *dailyPlanToggleAction // last w-key action for undo
	lastQueueToggle         *queueToggleAction     // last q-key action for undo
	lastDailyPlanToggleRedo *dailyPlanToggleAction // last undone w-key action for redo
	lastQueueToggleRedo     *queueToggleAction     // last undone q-key action for redo
	lastDailyPlanToggleAt   time.Time
	lastQueueToggleAt       time.Time
	lastDailyPlanRedoAt     time.Time
	lastQueueRedoAt         time.Time

	// Last repeatable prefix-chord action for . (dot) key replay.
	lastRepeatableAction *repeatableAction

	// Current project context
	currentProject *models.Project

	// Project stack tracks project IDs for stacked project view entries.
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
	statusUpdatedAt time.Time      // Guards newer status text from stale clear timers
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

	// Pending bulk note snapshots. Values are path-free and immutable after
	// capture so overlays cannot retain mutable view-cache pointers.
	pendingBulkNotes []noteMutationRef

	// Pending global note move operation
	pendingGlobalNoteMove *pendingNoteMove

	// Pending task creation parent (for template picker flow)
	pendingCreateTaskParent *models.Task

	// Pending task creation project (for template picker flow when the
	// caller targets a specific project, e.g. cross-project outliner).
	pendingCreateTaskProject *models.Project

	// Pending create-project continuation from system Inbox triage.
	pendingProjectMoveTask   *models.Task
	pendingProjectMoveSource *models.Project

	// Pending template for variable filler (between picker and task creator)
	pendingVarFillerTemplate *template.TaskTemplate

	// Pending description edit context (for text editor popup flow)
	pendingDescEdit *struct {
		Task    *models.Task
		Project *models.Project
	}

	// Pending continuation after the smart park prompt resolves.
	pendingParkNav *pendingParkNavigation

	// Pending sandbox reset confirmation
	pendingSandboxReset     bool
	pendingSandboxProjectID string
	pendingSandboxTaskCount int

	// Pending project archive/unarchive confirmation
	pendingProjectArchive        *models.Project
	pendingProjectArchiveArchive bool
	projectArchiveConfirm        ConfirmDialog

	// Pending shell-owned task deletion confirmation. The canonical
	// TaskDeleteRequestedMsg path stores the de-duplicated root refs plus the
	// originating presentation here, prompts through taskDeleteConfirm, and only
	// executes the atomic BulkCoordinator.Delete transaction on confirmation.
	pendingTaskDelete taskDeletePending
	taskDeleteConfirm ConfirmDialog

	// pendingInboxDeletes arms post-commit bookkeeping for inbox processors that
	// requested a canonical deletion, keyed by the per-request token minted in
	// confirmTaskDelete. Inbox processors are not TaskWorkspace-backed, so they defer
	// their advance/processed/cursor bookkeeping until the shell reports the delete
	// transaction committed (applyPendingInboxDelete); a rejected transaction clears
	// only the matching entry so the processor's queue stays untouched. Keying by
	// token (rather than a single slot) lets two overlapping same-surface deletes
	// each retain their own accounting until their matching result or error arrives,
	// in either completion order, and prevents an unrelated command that finishes
	// first from consuming an in-flight delete.
	pendingInboxDeletes map[uint64]pendingInboxDelete

	// pendingWeeklyReviewDeletes records the root-task count of each in-flight
	// Weekly Review-origin canonical deletion, keyed by the request token that armed
	// it. The Weekly Review completion summary has no post-commit callback of its
	// own, so its Deletions accounting is credited only after the shell reports the
	// exact batch committed (applyPendingWeeklyReviewDelete); a rejected transaction
	// or an unrelated result clears/skips only the matching entry so cancellation,
	// failures, and other deletes stay uncounted. Two overlapping Weekly Review
	// deletes are therefore each credited exactly once in either completion order.
	pendingWeeklyReviewDeletes map[uint64]int

	// deleteRequestSeq is a monotonic counter minted per canonical deletion in
	// confirmTaskDelete. The minted token rides the committing BatchTransactionResultMsg
	// / TransactionErrorMsg (stampDeleteRequestToken) so post-commit Inbox and Weekly
	// Review bookkeeping correlates with the exact result that originated it.
	deleteRequestSeq uint64

	// A trash restore commits the synced project/catalog data first, then repairs
	// machine-local references from the captured tombstone on the update loop.
	// Keep that payload until machine.json succeeds so a partial restore remains
	// retryable even though the durable trash entry has already been consumed.
	pendingTrashRestoreRecovery map[string]trashRestoreMachineRecovery

	// Synced workspace lifecycle commits are authoritative before their
	// machine-local cleanup runs. Retain an idempotent patch when machine.json
	// fails so the update loop can retry against fresh state without rolling
	// back or clobbering the already-committed workspace result.
	pendingMachineDomainRepair        machineDomainRepair
	machineDomainRepairRetryScheduled bool

	// CLI reference for AI assistant context
	cliReference string

	lastNavigatedTaskID string

	// Recent task visits for quick-switch (Alt+R)
	recentTaskVisits          []RecentTaskVisitEntry
	showQuickSwitch           bool
	quickSwitchCursor         int
	quickSwitchFilter         string
	quickSwitchFiltering      bool
	quickSwitchFiltered       []int   // indices into sortedQuickSwitchEntries
	quickSwitchTitlePositions [][]int // parallel to quickSwitchFiltered: fuzzy match positions in title

	// Pinned tasks popup state
	showPinnedPopup      bool
	pinnedPopupCursor    int
	pinnedPopupItems     []pinnedTaskInfo
	pinnedPopupFiltering bool
	pinnedPopupFilter    string
	pinnedPopupFiltered  []int          // indices into pinnedPopupItems
	pinnedPopupTitlePos  [][]int        // parallel to pinnedPopupFiltered: fuzzy match positions
	pinnedPreview        *OutlinerModel // cached preview outliner for highlighted pinned task's project
	pinnedPreviewID      string         // project ID the preview was built for
	pinnedPreviewTaskID  string         // task ID the preview cursor was set for

	// Proactive alert snapshots (ephemeral, regenerated on navigation)
	rawAlerts       *alerts.AlertSnapshot // unfiltered (before dismissal filter)
	proactiveAlerts *alerts.AlertSnapshot // filtered (after dismissals applied)
	dismissedSet    map[string]bool       // active dismissal keys

	// Message log: ring buffer of recent status messages for the log overlay.
	messageLog MessageLog

	// Ask applies can complete after their originating view is covered or
	// replaced. Retaining consumed process-wide IDs makes completion handling
	// exactly-once even if a stale or duplicate message is delivered later.
	handledAskApplyIDs map[uint64]struct{}

	// Keyboard enhancements reported by the terminal (Kitty protocol).
	keyboardEnhancements tea.KeyboardEnhancementsMsg

	// Quick note overlay (ctrl+. to add a comment without opening the full comments view)
	quickNoteOverlay *QuickNoteOverlay

	// Global project picker overlay (shown for project selection in global workflows)
	globalProjectPicker *GlobalProjectPicker

	// Unified Ctrl+X scope/administration picker (All Projects, Active, Archived, Trash)
	scopePicker *ScopePickerModel

	// Contextual project administration menu for a focused Outliner project header
	projectAdminMenu *ProjectAdminMenuModel

	// Global task picker overlay (shown for task selection in global workflows)
	globalTaskPicker *GlobalTaskPicker

	// Status quick picker overlay (shown as step 3 of global status update flow)
	statusQuickPicker *StatusQuickPicker

	// Section quick picker overlay (shown as step 4 of global task move flow)
	sectionQuickPicker *SectionQuickPicker

	// Section change quick picker overlay.
	sectionChangeQuickPicker *SectionQuickPicker

	// Intermediate state for the global task-move workflow.
	// Cleared on completion or cancellation.
	globalMoveState struct {
		sourceProject *models.Project
		sourceTask    *models.Task
		destProject   *models.Project
	}

	// Link type quick picker overlay.
	linkTypeQuickPicker *LinkTypeQuickPicker

	// Link mode quick picker overlay.
	linkModeQuickPicker *LinkModeQuickPicker

	// Priority quick picker overlay.
	priorityQuickPicker *PriorityQuickPicker

	// Due date quick picker overlay.
	dueDateQuickPicker *DueDateQuickPicker

	// Intermediate state for global linked-task creation.
	// Cleared on completion or cancellation.
	globalLinkedTaskState struct {
		sourceTask    *models.Task
		sourceProject *models.Project
		linkType      LinkType
	}

	// Peek task overlay (space toggles task detail peek)
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
	Source noteMutationRef
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)
