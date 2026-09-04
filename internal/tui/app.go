package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// AppModel is the main TUI model.
type AppModel struct {
	configManager *config.Manager

	// runtime is the startup-loaded snapshot of the three configuration domains
	// (catalog, planning, machine), their load-time raw
	// revisions, and the workspace data dir. It is the composition-root state the
	// shell holds as the sole configuration state:
	// views read domain data through its narrow accessors without render-time disk
	// loads, config-bearing mutations commit only the changed domain(s) through its
	// revision-checked commit seams, and each commit refreshes the snapshot so the views
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

	// Persister for project persistence operations (enables testing with mocks)
	persister coordinator.Persister

	// features are the application-backed coordinators the primary feature views
	// dispatch their mutations and queries through. They route persistence
	// through the application layer (one atomic ChangeSet per mutation) instead of
	// letting a view mutate and save its own model copy. See feature_coordinator.go.
	features coordinator.FeatureCoordinators

	// secondaryFeatures are the application-backed coordinators the secondary
	// views and overlays (bulk operations, attachments, and tags)
	// dispatch their mutations through. Each depends on
	// a narrow host port satisfied by the same application service, so an overlay
	// routes persistence through one atomic ChangeSet (or config write) instead of
	// a direct saveProjectSync / configManager.Save call. See
	// feature_coordinator_secondary.go and its per-coordinator files.
	secondaryFeatures coordinator.SecondaryCoordinators

	// manageHub is the sole live Manage destination for scoped collections and
	// contexts.
	manageHub *projecthub.ManageHubModel

	// taskComposer is the sole live compact/expanded task mutation overlay.
	// Capture/form presentation IDs remain observations for embedded modes.
	taskComposer *taskform.TaskComposerModel

	// appService is the same application service the feature coordinators wrap,
	// retained directly for the few AppModel-owned overlays that mutate
	// synchronously in Update() and need the typed result inline. Routing through
	// it keeps those mutations atomic
	// and ledger-undoable without a direct raw save.
	appService *app.Service

	// viewNav is the view and navigation cluster: the authoritative route and
	// overlay stacks, the renderer registry, and the terminal geometry both are
	// sized against. See view_navigation_controller.go.
	viewNav        viewNavigationController
	startupMessage tea.Msg

	// pendingLaunchLens is the lens the machine-local active-lens pointer named
	// when this session was constructed, held here because the launch reset
	// clears that pointer before any message can read it. It is drained exactly
	// once, after the launch population lands, and forgotten either way.
	pendingLaunchLens *config.NamedLens

	// noteIndexRows is the workspace-wide note snapshot the Outliner's search
	// tier ranks. It is shell-owned because NotesAtLocations reads disk and the
	// tier is scored on every keystroke: the read happens once per invalidation,
	// asynchronously, and the Outliner only ever reads the retained result.
	// noteIndexGeneration discards a superseded read, and noteIndexOwners is the
	// owner-catalog fingerprint that decides whether an availability change owes
	// a reload at all.
	noteIndexRows       []notes.LocatedNoteRow
	noteIndexGeneration int
	noteIndexOwners     string

	operationRegistry *operation.OperationRegistry
	operationExecutor *operation.OperationExecutor

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	helpView      *help.HelpViewModel
	crossOutliner *outliner.OutlinerModel

	// Focus view session state (persists across view navigation within a session)
	nextOrder    []outliner.PlanningTask
	nextOrderIDs map[taskref.TaskIdentity]bool

	// Today session state (persists across view navigation within a session)
	today    []outliner.PlanningTask
	todayIDs map[taskref.TaskIdentity]bool

	// workspacePulse caches the cross-project operational counts surfaced in the
	// title row. It is recomputed after committed count-changing mutations so
	// paint paths can read it without an extra full-tree walk.
	workspacePulse pulse.WorkspacePulse
	// completionActivity is refreshed through the application query alongside
	// other operational title state, then injected into the Outliner before paint.
	// Keeping the snapshot out of View avoids render-time filesystem reads.
	completionActivity app.CompletionActivity
	// outstandingConflicts is how many bundles `.tt/conflicts/` holds. It is
	// deliberately not part of WorkspacePulse: that ladder is a closed ordinal and
	// ComputeWorkspacePulse has no seam for filesystem-derived state. The status
	// strip carries this instead, off the same 60s tick, and the count is cached
	// here so no render reads the directory.
	outstandingConflicts int
	// hideAdvisories suppresses every ambient advisory for this session: the
	// conflict count and the load-failure notices alike. See advisories.go for
	// what counts as one, and advisories_shell.go for how the machine-local
	// setting and the `--no-advisories` flag both feed this value.
	hideAdvisories bool

	lastToggleUndoKind toggleUndoKind
	lastToggleRedoKind toggleUndoKind

	lastTodayToggleAt time.Time
	lastQueueToggleAt time.Time
	lastTodayRedoAt   time.Time
	lastQueueRedoAt   time.Time

	// Current project context
	currentProject *models.Project

	// Project stack tracks project IDs for stacked project view entries.
	// When the project picker pushes a new project tree, the previous project ID
	// is saved so it can be restored when navigating back.
	projectStack []string

	// State
	err             error
	quitting        bool
	statusMessage   string         // Temporary status message (auto-clears after timeout)
	statusIsInfo    bool           // When true, render status with InfoStatusStyle instead of SuccessStyle
	statusUpdatedAt time.Time      // Guards newer status text from stale clear timers
	statusUndoCb    func() tea.Cmd // When non-nil and not expired, pressing `u` invokes this callback
	statusExpiresAt time.Time      // Deadline after which statusUndoCb is no longer valid
	// statusTransactionID correlates the active recovery toast with a committed
	// workspace transaction so delayed origin-history feedback cannot decorate a
	// newer status.
	statusTransactionID    string
	statusRecoveryFallback string
	statusSeq              uint64              // Bumped by every status setter; drives the footer divider flash
	statusSeverity         footerFlashSeverity // Severity of the newest status text
	footerFlash            footerFlashState
	footerFlashSeq         uint64

	// Pending narrow daily task edit. Stable identity is captured when the editor
	// opens so cursor movement or a view rebuild cannot retarget the save.
	pendingTaskTextEdit *struct {
		Identity  taskref.TaskIdentity
		Operation app.TaskOperation
		Original  string
	}

	// Pending project archive/unarchive confirmation
	pendingProjectArchive        *models.Project
	pendingProjectArchiveArchive bool
	projectArchiveConfirm        chrome.ConfirmDialog

	// Pending project delete confirmation. Deleting is the one project act that
	// removes files, so it keeps its own dialog rather than sharing the archive
	// one: the prompt counts the cascade the archive sentence never names.
	pendingProjectDelete *models.Project
	projectDeleteConfirm chrome.ConfirmDialog

	// Pending project park confirmation. Only a tag edit that newly adds the
	// reserved parked tag prompts; every other project tag applies straight away.
	pendingProjectTags       *models.Project
	pendingProjectTagsSet    []string
	pendingProjectBulkTags   *projectBulkTagEdit
	pendingProjectEditorSave *projecthub.ProjectEditorSave
	projectParkConfirm       chrome.ConfirmDialog

	// projectTagActionSubject is the tag the vocabulary sheet edits. It lives
	// here rather than in the sheet's frozen value because a nested value step
	// overwrites that value on its way back to the executor.
	projectTagActionSubject string

	// taskMutation is the task-mutation cluster: the shell-owned task deletion
	// and outline-apply confirmations, each holding the pending value its
	// transaction is submitted from once the user confirms. The transactions
	// themselves stay on AppModel. See task_mutation_controller.go.
	taskMutation taskMutationController

	// The contextual action flow is the only generic two-level overlay:
	// action sheet -> value step. The sheet is host-agnostic; the host it was
	// opened for travels on every message it emits. The value step is a
	// ListSurface picker, or a text prompt when the value is typed.
	contextActionSheet   *listsurface.ContextActionSheetModel
	operationValuePicker *OperationValuePickerModel
	operationTextPrompt  *OperationTextPromptModel

	// The named-lens picker sits below that flow rather than inside it: it is
	// the surface an action sheet is opened *from*, so it stays mounted beneath
	// the sheet, prompt, and confirmation a lens action nests above it.
	lensPicker *lens.NamedLensPickerModel

	// The conflict browser sits beside the picker rather than inside that flow:
	// it registers no operations, so nothing ever nests above it, and Esc is the
	// whole of its lifecycle.
	conflictBrowser *conflict.ConflictBrowserModel

	// A destructive contextual action prompts here and keeps its frozen request
	// so cancellation returns to the sheet with nothing executed.
	operationConfirm        chrome.ConfirmDialog
	pendingOperationRequest operation.OperationExecutionRequest

	// Keys already pressed toward an unfinished chord. A non-empty prefix owns
	// the footer as a which-key row until the chord completes or is abandoned.
	pendingChord []string
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)
