package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// inboxBackend abstracts the differences between local and global inbox processors.
// Each model implements this interface to provide view-specific data access,
// persistence, rendering strings, and behavioral hooks.
type inboxBackend interface {
	// Data access
	taskCount() int
	taskRef(index int) (task *models.Task, project *models.Project)

	// Messages
	closeMsg(processed, skipped int) tea.Msg
	handleSavedMsg(msg tea.Msg) (matched bool, err error)

	// Persistence
	saveCmd() tea.Cmd
	// saveProjectCmd saves the given project. Used by undo to persist
	// changes to the action's project even when the cursor has auto-advanced
	// onto a different project (global processor only). For the local
	// processor, the parameter is the same project the cursor sits on.
	saveProjectCmd(project *models.Project) tea.Cmd
	reloadCmd() tea.Cmd
	rebuildTasks()

	// Behavioral hooks
	markDeletedProcessed() bool
	shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool
	shouldProcessOnStateChange(newState models.WorkflowState) bool

	// findTaskByID locates a task by ID (and optional project ID hint) so undo
	// can restore an action's task even after auto-advance has moved the cursor
	// away. The projectID hint is authoritative for the global processor; the
	// local processor ignores it because it only owns a single project.
	findTaskByID(taskID, projectID string) (task *models.Task, project *models.Project)

	// projectByID returns the in-memory project pointer for a logical id so a
	// ledger-backed undo/redo can reload exactly the projects the restored
	// transaction touched. Returns nil when the processor does not own that
	// project.
	projectByID(projectID string) *models.Project

	// Rendering
	showProjectInHeader() bool
	headerTitle() string
	headerSubtitleExtra() string
	emptyStateText() string
	emptyInboxText() string
	returnHint() string
	viewType() PresentationID
	helpViewName() string
}

// inboxCore holds all state and logic shared between InboxProcessorModel and
// GlobalInboxProcessorModel. Each model embeds this struct and sets the backend
// field to itself in its constructor.
type inboxCore struct {
	BaseView
	InteractiveViewState
	configManager *config.Manager
	// runtime backs the domain-native global section-color read (Settings).
	runtime *config.RuntimeState

	currentIndex      int
	processedIDs      map[string]bool
	skippedIDs        map[string]bool
	processingHistory []processingAction
	// redoHistory holds actions popped by undo so a subsequent ctrl+r can replay
	// them. Any newly recorded action clears it, matching the linear Vim-style
	// contract the workspace ledger also enforces. Skip actions redo locally;
	// stateful actions redo through the ledger.
	redoHistory []processingAction

	// pendingFocusID, when non-empty, names the task the cursor should land on
	// after the next rebuildTasks. advanceToNext sets it to the new current
	// task's ID so a subsequent rebuild (triggered by InboxProjectSavedMsg)
	// can re-anchor instead of letting clamp drift past the intended task.
	// Undo also sets it so the restored task gets focus after its project is
	// reloaded from disk.
	pendingFocusID string

	// pendingDeletes holds the post-commit focus/auto-close metadata for each task
	// this processor asked the shell to delete but whose transaction has not yet
	// committed, keyed by task ID. It gates applyCommittedDelete so post-commit
	// bookkeeping runs exactly once per deleted task. Each entry captures the
	// post-delete cursor target computed against the intact queue at request time
	// (see requestDelete), so the post-commit advance matches the legacy in-memory
	// delete instead of skipping the natural next task. Keying by task ID (rather
	// than a single slot) lets two overlapping deletes — delete, navigate, delete
	// again — each retain their own focus target regardless of which result lands
	// first.
	pendingDeletes map[string]inboxPendingDelete

	backend inboxBackend
	model   tea.Model // pointer to the outer (embedding) model, for Update returns
}

// inboxPendingDelete captures the post-delete cursor target computed against the
// intact queue at request time, so the post-commit advance matches the legacy
// in-memory delete instead of skipping the natural next task. focusID names the
// task the cursor should land on once the deleted task leaves the queue.
//
// Auto-close is deliberately NOT captured here: back-to-back deletion of the
// final two tasks would each snapshot willClose=false (each still sees the
// other's task queued), so closure is recomputed at apply time against the
// rebuilt committed queue plus the remaining in-flight deletes (see
// shouldCloseAfterDelete), which closes the processor exactly once.
type inboxPendingDelete struct {
	focusID string
}

// processingAction records an action for undo support.
type processingAction struct {
	taskID       string
	projectID    string
	actionType   string
	timestamp    time.Time
	prevSnapshot *inboxSnapshot
}

// initCore initializes the shared core fields. Call from the outer model's constructor.
func (c *inboxCore) initCore(backend inboxBackend, model tea.Model, manager *config.Manager, runtime *config.RuntimeState) {
	c.configManager = manager
	c.runtime = runtime
	c.currentIndex = 0
	c.processedIDs = make(map[string]bool)
	c.skippedIDs = make(map[string]bool)
	c.processingHistory = []processingAction{}
	c.redoHistory = []processingAction{}
	c.backend = backend
	c.model = model

	km := c.buildKeyMap()
	c.Dispatcher = keymap.NewDispatcher(km)
}
