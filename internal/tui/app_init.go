package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// WithPersister sets a custom Persister for the AppModel.
// This is primarily used for testing with mock implementations.
func WithPersister(p Persister) AppOption {
	return func(app *AppModel) {
		app.persister = p
	}
}

// WithCLIReference sets the CLI reference text for AI assistant context.
func WithCLIReference(ref string) AppOption {
	return func(app *AppModel) {
		app.cliReference = ref
	}
}

// NewAppModel creates a new TUI application model.
func NewAppModel(cfg *config.Config, manager *config.Manager, opts ...AppOption) *AppModel {
	app := &AppModel{
		config:                   cfg,
		configManager:            manager,
		persister:                NewProjectService(),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		views:                    NewViewRegistry(),
		commandRegistry:          NewCommandRegistry(),
		spinner:                  NewSpinner(),
		navHistory:               NewNavHistory(),
		lastProjectView:          defaultProjectViewType(cfg),
		projectLastView:          make(map[string]ViewType),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
		priorityViewFilterCache:  make(map[string]*PriorityViewCache),
		focusQueue:               []FocusTask{},
		focusQueueIDs:            make(map[string]bool),
		focusDismissedIDs:        make(map[string]bool),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}

	// Hydrate focus queue from persisted config (only from active projects)
	activeProjects := cfg.GetActiveProjects()
	for _, item := range cfg.GetFocusQueue() {
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil && !task.Status.IsComplete() {
				app.focusQueue = append(app.focusQueue, FocusTask{Task: task, Project: p})
				app.focusQueueIDs[task.ID] = true
			}
			break
		}
	}
	// Hydrate dismissed IDs
	for _, id := range cfg.GetFocusDismissedIDs() {
		app.focusDismissedIDs[id] = true
	}

	// Hydrate recent task visits from persisted config
	for _, item := range cfg.GetRecentTaskVisits() {
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				app.recentTaskVisits = append(app.recentTaskVisits, RecentTaskVisitEntry{Task: task, Project: p, Pinned: item.Pinned})
			}
			break
		}
	}

	// Ensure sandbox project exists
	if cfg.EnsureSandboxProject() {
		saveConfigWarn(manager, cfg, "sandbox project creation")
	}

	// Initialize dashboard and register it
	app.dashboard = NewDashboardModel(cfg, manager)
	app.views.Register(ViewDashboard, app.dashboard)

	// Record initial dashboard view in navigation history
	app.navHistory.Record(NavEntry{ViewType: ViewDashboard})

	// Register global commands
	app.registerGlobalCommands()

	// Load current project if set
	if current := cfg.GetCurrentProject(); current != nil {
		app.currentProject = current
	}

	ApplyTheme(NewThemeConfig(cfg))

	return app
}

// makePriorityHandler creates a command handler that sets task priority.
func (m *AppModel) makePriorityHandler(priority int) func() tea.Msg {
	return func() tea.Msg {
		if m.projectTree == nil {
			return nil
		}
		visible := m.projectTree.visibleNodes()
		if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
			for id := range m.projectTree.selectedTasks {
				m.projectTree.project.MutateTask(id, func(t *models.Task) {
					t.Priority = priority
				})
			}
			m.projectTree.clearSelection()
			m.projectTree.buildTree()
			return nil
		}
		if CursorInBounds(m.projectTree.cursor, len(visible)) {
			task := visible[m.projectTree.cursor].Task
			m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
				t.Priority = priority
			})
			return TaskPropertyChangedMsg{Task: task}
		}
		return nil
	}
}

// registerGlobalCommands registers commands that are available from any view.
func (m *AppModel) registerGlobalCommands() {
	_ = m.commandRegistry.RegisterBatch(m.globalCommands())
	_ = m.commandRegistry.RegisterBatch(m.filterCommands())
	_ = m.commandRegistry.RegisterBatch(m.viewCommands())
	_ = m.commandRegistry.RegisterBatch(m.navCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskCommands())
	_ = m.commandRegistry.RegisterBatch(m.projectCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskActionCommands())
	_ = m.commandRegistry.RegisterBatch(m.viewModeCommands())
	_ = m.commandRegistry.RegisterBatch(m.statusCommands())
	_ = m.commandRegistry.RegisterBatch(m.dueDateCommands())
	_ = m.commandRegistry.RegisterBatch(m.notesCommands())
	_ = m.commandRegistry.RegisterBatch(m.otherCommands())
	_ = m.commandRegistry.RegisterBatch(m.priorityCommands())
	_ = m.commandRegistry.RegisterBatch(m.sortCommands())
	_ = m.commandRegistry.RegisterBatch(m.structureCommands())
	_ = m.commandRegistry.RegisterBatch(m.sectionSendCommands())
	_ = m.commandRegistry.RegisterBatch(m.linkedTaskCommands())
	_ = m.commandRegistry.RegisterBatch(m.multiSelectCommands())
	_ = m.commandRegistry.RegisterBatch(m.foldCommands())
	_ = m.commandRegistry.RegisterBatch(m.filterBarCommands())
	_ = m.commandRegistry.RegisterBatch(m.notesViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.blockersCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskCommentsCommands())
	_ = m.commandRegistry.RegisterBatch(m.undoHistoryCommands())
	_ = m.commandRegistry.RegisterBatch(m.globalTagManagerCommands())
	_ = m.commandRegistry.RegisterBatch(m.attachmentCommands())
	_ = m.commandRegistry.RegisterBatch(m.focusViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.dailyReviewCommands())
	_ = m.commandRegistry.RegisterBatch(m.priorityViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.inboxProcessorCommands())
	_ = m.commandRegistry.RegisterBatch(m.dueDateOverviewCommands())
	_ = m.commandRegistry.RegisterBatch(m.workflowRulesCommands())
	_ = m.commandRegistry.RegisterBatch(m.sidebarCommands())
	_ = m.commandRegistry.RegisterBatch(m.scoringCommands())
	_ = m.commandRegistry.RegisterBatch(m.allTasksCommands())
	_ = m.commandRegistry.RegisterBatch(m.outlinerCommands())
}

// getGlobalHelpItems returns help items that apply across all views.
func getGlobalHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "?", Description: "Show help", Category: "General"},
		{Key: "q", Description: "Quit / go back", Category: "General"},
		{Key: "esc", Description: "Go back / cancel", Category: "General"},
		{Key: "ctrl+c", Description: "Quit application", Category: "General"},
		{Key: "/", Description: "Search projects", Category: "Navigation"},
		{Key: ";/", Description: "Global search", Category: "Navigation"},
		{Key: ":", Description: "Command palette", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor down/up", Category: "Navigation"},
		{Key: "↑/↓", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "g/G", Description: "Go to first/last item", Category: "Navigation"},
		{Key: "Enter", Description: "Select / confirm", Category: "Actions"},
		{Key: ";n", Description: "Notification Center", Category: "Navigation"},
		{Key: "ctrl+,", Description: "Settings", Category: "Navigation"},
		{Key: ";d", Description: "Daily Review", Category: "Navigation"},
		{Key: ";w", Description: "Weekly Review", Category: "Navigation"},
		{Key: "alt+o", Description: "History back", Category: "Navigation"},
		{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		{Key: "|", Description: "Recent tasks", Category: "Navigation"},
	}
}

const periodicSaveInterval = 60 * time.Second

func (m *AppModel) Init() tea.Cmd {
	return tea.Tick(periodicSaveInterval, func(time.Time) tea.Msg {
		return periodicSaveMsg{}
	})
}
