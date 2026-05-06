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
	reloadCmd() tea.Cmd
	rebuildTasks()
	deleteFromData(task *models.Task, project *models.Project)

	// Behavioral hooks
	markDeletedProcessed() bool
	shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool
	shouldProcessOnStatusChange(newStatus models.TaskStatus) bool

	// Rendering
	showProjectInHeader() bool
	headerTitle() string
	headerSubtitleExtra() string
	emptyStateText() string
	emptyInboxText() string
	returnHint() string
	viewType() ViewType
	helpViewName() string
}

// inboxCore holds all state and logic shared between InboxProcessorModel and
// GlobalInboxProcessorModel. Each model embeds this struct and sets the backend
// field to itself in its constructor.
type inboxCore struct {
	BaseView
	InteractiveViewState
	config        *config.Config
	configManager *config.Manager

	currentIndex      int
	processedIDs      map[string]bool
	skippedIDs        map[string]bool
	processingHistory []processingAction

	backend inboxBackend
	model   tea.Model // pointer to the outer (embedding) model, for Update returns
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
func (c *inboxCore) initCore(backend inboxBackend, model tea.Model, cfg *config.Config, manager *config.Manager) {
	c.config = cfg
	c.configManager = manager
	c.currentIndex = 0
	c.processedIDs = make(map[string]bool)
	c.skippedIDs = make(map[string]bool)
	c.processingHistory = []processingAction{}
	c.backend = backend
	c.model = model

	km := c.buildKeyMap()
	c.Dispatcher = keymap.NewDispatcher(km)
	c.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return c.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := c.confirm.HandleKey(key); handled {
					if confirmed {
						return c.model, c.executeDelete(), true
					}
					c.statusMessage = ""
					return c.model, nil, true
				}
				return nil, nil, false
			},
		},
	}
}
