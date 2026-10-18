package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/entityops"
	"github.com/sglavoie/tt/internal/tui/filterops"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/lensops"
	"github.com/sglavoie/tt/internal/tui/notesops"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/projectops"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/workspace"
)

// AppModel is the main TUI model.
type AppModel struct {
	checkInFeedback checkInFeedbackController
	checkInSetup    *checkInSetupPass

	// interaction is which user interaction owns the asynchronous work still in
	// flight, so a refused save hands its draft and frozen targets back to the
	// editor that submitted them and to nothing the user moved on to. It is
	// separate from checkInFeedback on purpose: that note rides whatever line is
	// speaking and takes no keyboard, so it asks nothing of ownership. See
	// app_update.go.
	interaction interactionOwnership
	// domain is the domain commit and configuration cluster: the startup-loaded
	// snapshot of the catalog, planning and machine domains, the manager and
	// application service every commit against them travels through, the workspace
	// history handle those commits record into, and the session-stable project
	// cache the loader fills beside them. Every committer stays an AppModel
	// method. See domain_controller.go.
	domain domainController

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

	// hostedPanelHost records which presentation the shell last shrank to make
	// room for a bounded bottom panel, so closing the panel hands that one view
	// its whole budget back. See app_hosted_panel.go.
	hostedPanelHost hostedPanelHostRecord

	// shellFrame records which presentation the shell last told how tall its
	// footer is, so the view that stops being active is told the footer is no
	// longer its to reserve for. See app_hosted_panel.go.
	shellFrame shellFrameRecipient

	// viewNav is the view and navigation cluster: the authoritative route and
	// overlay stacks, the renderer registry, and the terminal geometry both are
	// sized against. See view_navigation_controller.go.
	viewNav viewNavigationController

	// lifecycle is the session-lifecycle cluster: the message the composition
	// root owes the first frame, the flag a quit sets so the final paint renders
	// nothing, and the scheduling of the one recurring app timer. RunApp, the
	// composition root, and teardown stay outside it, because none of them
	// retains state. See lifecycle_controller.go.
	lifecycle lifecycleController

	// notes is the notes cluster: the workspace-wide note snapshot the
	// Outliner's search tier ranks, the generation that discards a superseded
	// read, the owner-catalog fingerprint that decides whether an availability
	// change owes a reload at all, and the note operations that surface is
	// driven by. See notesops/controller.go.
	notes notesops.Controller

	// operations is the operation-wiring cluster: the descriptor registry, the
	// invocation executor, the contextual action sheet and the value or
	// confirmation step one invocation can open above it, and the chord prefix a
	// key path has accumulated so far. The behavior lives in operationhost; the
	// fifteen registrars and every handler stay AppModel methods, because the
	// domains they commit against are the shell's. See operationhost/controller.go.
	operations operationhost.Controller

	// entities is the task-owned entity cluster: attachments, comments, and
	// checklist rows. Each family's descriptors, action sheet, and coordinator
	// calls live in entityops beside the panels they act on; the shell holds the
	// controller and satisfies its narrow port. See shell_ports.go.
	entities entityops.Controller

	// filters is the facet and sort cluster. The behavior lives in filterops
	// beside the Outliner query it edits; the shell only holds the controller and
	// satisfies its narrow port. See shell_ports.go.
	filters filterops.Controller

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	helpView      *help.HelpViewModel
	crossOutliner *outliner.OutlinerModel
	helpSession   help.Session

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
	// pulseLoader is the pulse's own retained snapshot loader. It is read only
	// from Update, never from a command goroutine, which is what lets it outlive
	// one recount without sharing the Outliner's loader across goroutines.
	pulseLoader *workspace.Loader
	// completionActivity is refreshed through the application query alongside
	// other operational title state, then injected into the Outliner before paint.
	// Keeping the snapshot out of View avoids render-time filesystem reads.
	completionActivity app.CompletionActivity

	// feedback is the status and feedback cluster: the shell's own status line
	// and error, the undo callback and transaction correlation a recovery toast
	// carries, the footer divider flash a change to any of them arms, and the
	// two ambient inputs the status strip reads — the outstanding-conflict count
	// and whether advisories are shown at all. Every setter stays an AppModel
	// method. See feedback_controller.go.
	feedback feedbackController

	// externalReload is the workspace as it stood at the last focus loss, and
	// whether a change made elsewhere is still owed. See lifecycle_controller.go.
	externalReload externalReloadState
	automaticSync  automaticSyncState

	// notifications is a --notify run's arrival reporting. See lifecycle_controller.go.
	notifications notificationState

	// control is the session's link to a host's control socket, when tt runs
	// with one. See lifecycle_controller.go.
	control controlState

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

	// Pending narrow daily task edit. Stable identity is captured when the editor
	// opens so cursor movement or a view rebuild cannot retarget the save.
	pendingTaskTextEdit *struct {
		Identity  taskref.TaskIdentity
		Operation app.TaskOperation
		Original  string
	}

	// project is the project cluster: the archive lifecycle, the delete cascade,
	// the one park confirmation the tag picker, the bulk tag actions and the
	// project editor all raise, and the tag the vocabulary sheet edits. Every
	// commit stays an AppModel method. See projectops/project_controller.go.
	project projectops.Controller

	// taskMutation is the task-mutation cluster: the shell-owned task deletion
	// and outline-apply confirmations, each holding the pending value its
	// transaction is submitted from once the user confirms. The transactions
	// themselves stay on AppModel. See task_mutation_controller.go.
	taskMutation taskMutationController

	// dueClocks is the session's memory of the wall clocks deadline stepping
	// cleared, so stepping back out of unset restores the time a task had instead
	// of inventing one. It is keyed by project-qualified task address and held
	// here rather than on the Outliner because the transactions it listens to —
	// bulk edits, inline and external editor saves, outline applies, ledger
	// restores — are the shell's, not one surface's. Nothing about it is
	// persisted: a restart starts empty. See due_clock_memory.go.
	dueClocks coordinator.DueClockController

	// undoFocus is the session's memory of which single task each committed
	// transaction was aimed at, so undoing one can put the cursor back on the work
	// it restored instead of inferring a target from a reloaded workspace. It sits
	// beside dueClocks for the same reason and on the same terms: only the shell
	// sees both the commit that names a target and the ledger restore that spends
	// it, and nothing about it is persisted, so a restart correlates nothing. See
	// handler_task_crud.go.
	undoFocus coordinator.UndoFocusController

	// dueSteps serializes the deadline gestures the + and - keys record. It sits
	// beside dueClocks for the same reason: a burst of keypresses is one chain of
	// transactions, and only the shell sees both the keypress that freezes a
	// gesture's targets and the committed result that releases the next one. See
	// due_clock_memory.go.
	dueSteps coordinator.DueStepQueue

	// vocabularyRename holds the frozen plan the two filter pickers' rename step
	// is showing. It lives here rather than on the picker because a preview pins
	// a whole workspace census, which is the shell's to hold and to release. See
	// vocabulary_rename_host.go.
	vocabularyRename vocabularyRenameSession

	// lenses is the named-lens cluster: the picker, which sits below the
	// contextual action flow rather than inside it because it is the surface an
	// action sheet is opened *from*; the lens the machine-local pointer asked
	// this session to reopen with; and the injection of the machine-local write
	// that remembers which lens is live. The write itself stays an AppModel
	// method. See lensops/controller.go.
	lenses lensops.Controller

	// The conflict browser sits beside the picker rather than inside that flow:
	// it registers no operations, so nothing ever nests above it, and Esc is the
	// whole of its lifecycle.
	conflictBrowser *conflict.ConflictBrowserModel
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)
