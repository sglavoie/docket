package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/appearance"
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
		appearanceDetector:       appearance.NewDetector(),
		persister:                NewProjectService(),
		viewStack:                []ViewType{ViewDashboard},
		sessionStats:             sessionStats{sessionStart: time.Now()},
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
		kanbanViewCache:          make(map[string]*KanbanViewCache),
		focusQueue:               []FocusTask{},
		focusQueueIDs:            make(map[string]bool),
		focusDismissedIDs:        make(map[string]bool),
		dailyPlan:                []FocusTask{},
		dailyPlanIDs:             make(map[string]bool),
		dailyPlanCarriedIDs:      make(map[string]bool),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}

	activeProjects := cfg.GetActiveProjects()
	app.hydrateFocusQueue(cfg, manager, activeProjects)
	app.hydrateDailyPlan(cfg, manager, activeProjects)
	app.hydrateRecentVisits(cfg, manager, activeProjects)

	// Hydrate dismissed IDs
	for _, id := range cfg.GetFocusDismissedIDs() {
		app.focusDismissedIDs[id] = true
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
	app.dashboard.SetAccentColor(AccentDashboard)

	app.openLoops = ComputeOpenLoops(cfg, app.dailyPlanIDs)

	return app
}

// hydrateFocusQueue restores the focus queue from persisted config, pruning stale entries.
func (app *AppModel) hydrateFocusQueue(cfg *config.Config, manager *config.Manager, activeProjects []*models.Project) {
	queueChanged := false
	var updatedQueue []config.FocusQueueItem
	for _, item := range cfg.GetFocusQueue() {
		found := false
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
				updatedQueue = append(updatedQueue, item)
				found = true
			}
			break
		}
		if !found {
			queueChanged = true
		}
	}
	if queueChanged {
		if updatedQueue == nil {
			updatedQueue = []config.FocusQueueItem{}
		}
		cfg.SetFocusQueue(updatedQueue)
		saveConfigWarn(manager, cfg, "focus queue pruning")
	}
}

// hydrateDailyPlan restores the daily plan from persisted config with carry-over logic.
func (app *AppModel) hydrateDailyPlan(cfg *config.Config, manager *config.Manager, activeProjects []*models.Project) {
	today := time.Now().Format("2006-01-02")
	var updatedPlan []config.DailyPlanItem
	planChanged := false
	for _, item := range cfg.GetDailyPlan() {
		var foundTask *models.Task
		var foundProject *models.Project
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				foundTask = task
				foundProject = p
			}
			break
		}
		if foundTask == nil || foundProject == nil {
			planChanged = true
			continue
		}
		if item.PlannedDate < today {
			if foundTask.Status.IsComplete() {
				planChanged = true
				continue
			}
			item.PlannedDate = today
			app.dailyPlanCarried++
			app.dailyPlanCarriedIDs[foundTask.ID] = true
			planChanged = true
		}
		updatedPlan = append(updatedPlan, item)
		app.dailyPlan = append(app.dailyPlan, FocusTask{Task: foundTask, Project: foundProject})
		app.dailyPlanIDs[foundTask.ID] = true
	}
	if planChanged {
		if updatedPlan == nil {
			updatedPlan = []config.DailyPlanItem{}
		}
		cfg.SetDailyPlan(updatedPlan)
		saveConfigWarn(manager, cfg, "daily plan carry-over")
	}
}

// hydrateRecentVisits restores recent task visits from persisted config, pruning stale entries.
func (app *AppModel) hydrateRecentVisits(cfg *config.Config, manager *config.Manager, activeProjects []*models.Project) {
	visitsChanged := false
	var updatedVisits []config.RecentTaskVisit
	for _, item := range cfg.GetRecentTaskVisits() {
		found := false
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				app.recentTaskVisits = append(app.recentTaskVisits, RecentTaskVisitEntry{Task: task, Project: p, Pinned: item.Pinned})
				updatedVisits = append(updatedVisits, item)
				found = true
			}
			break
		}
		if !found {
			visitsChanged = true
		}
	}
	if visitsChanged {
		if updatedVisits == nil {
			updatedVisits = []config.RecentTaskVisit{}
		}
		cfg.SetRecentTaskVisits(updatedVisits)
		saveConfigWarn(manager, cfg, "recent task visits pruning")
	}
}

// makePriorityHandler creates a command handler that sets task priority.
func (m *AppModel) makePriorityHandler(priority int) func() tea.Msg {
	return func() tea.Msg {
		if m.projectTree == nil {
			return nil
		}
		visible := m.projectTree.visibleNodes()
		if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
			taskIDs := sortedSelectedTaskIDs(m.projectTree.selectedTasks)
			for id := range m.projectTree.selectedTasks {
				m.projectTree.project.MutateTask(id, func(t *models.Task) {
					t.Priority = priority
				})
			}
			m.projectTree.clearSelection()
			m.projectTree.buildTree()
			return bulkTaskPropertyChangedWithUndo("Set Priority", fmt.Sprintf("P%d: %d tasks", priority, len(taskIDs)), taskIDs)
		}
		if CursorInBounds(m.projectTree.cursor, len(visible)) {
			task := visible[m.projectTree.cursor].Task
			m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
				t.Priority = priority
			})
			return taskPropertyChangedWithUndo(task, "Set Priority")
		}
		return nil
	}
}

// commandGroups returns every command batch in the order they are registered.
// Tests use this to rebuild the registry and assert palette invariants without
// drifting from the production registration order.
func (m *AppModel) commandGroups() [][]Command {
	return [][]Command{
		m.globalCommands(),
		m.filterCommands(),
		m.viewCommands(),
		m.navCommands(),
		m.taskCommands(),
		m.projectCommands(),
		m.taskActionCommands(),
		m.viewModeCommands(),
		m.statusCommands(),
		m.dueDateCommands(),
		m.notesCommands(),
		m.otherCommands(),
		m.priorityCommands(),
		m.sortCommands(),
		m.structureCommands(),
		m.sectionSendCommands(),
		m.linkedTaskCommands(),
		m.multiSelectCommands(),
		m.foldCommands(),
		m.filterBarCommands(),
		m.notesViewCommands(),
		m.blockersCommands(),
		m.taskCommentsCommands(),
		m.globalTagManagerCommands(),
		m.attachmentCommands(),
		m.focusViewCommands(),
		m.dailyReviewCommands(),
		m.priorityViewCommands(),
		m.inboxProcessorCommands(),
		m.dueDateOverviewCommands(),
		m.workflowRulesCommands(),
		m.sidebarCommands(),
		m.scoringCommands(),
		m.allTasksCommands(),
		m.outlinerCommands(),
		m.exportCommands(),
	}
}

// registerGlobalCommands registers commands that are available from any view.
func (m *AppModel) registerGlobalCommands() {
	for _, group := range m.commandGroups() {
		_ = m.commandRegistry.RegisterBatch(group)
	}
}

// getGlobalHelpItems returns help items that apply across all views.
func getGlobalHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "?", Description: "Show help", Category: "General"},
		{Key: "esc", Description: "Go back / cancel", Category: "General"},
		{Key: "ctrl+c", Description: "Quit application", Category: "General"},
		{Key: "/", Description: "Search projects", Category: "Navigation"},
		{Key: "ctrl+/", Description: "Global search", Category: "Navigation"},
		{Key: ":", Description: "Command palette", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor down/up", Category: "Navigation"},
		{Key: "↑/↓", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "g/G", Description: "Go to first/last item", Category: "Navigation"},
		{Key: "Enter", Description: "Select / confirm", Category: "Actions"},
		{Key: ";n", Description: "Notification Center", Category: "Navigation"},
		{Key: "ctrl+,", Description: "Settings", Category: "Navigation"},
		{Key: ";d", Description: "Daily Review", Category: "Navigation"},
		{Key: ";r", Description: "Weekly Review", Category: "Navigation"},
		{Key: "alt+o", Description: "History back", Category: "Navigation"},
		{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		{Key: "|", Description: "Recent tasks", Category: "Navigation"},
	}
}

const periodicSaveInterval = 60 * time.Second
const appearanceCheckInterval = 30 * time.Second

func (m *AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.Tick(periodicSaveInterval, func(time.Time) tea.Msg {
			return periodicSaveMsg{}
		}),
	}
	if m.config.GetAutoTheme() {
		cmds = append(cmds, tea.Tick(appearanceCheckInterval, func(time.Time) tea.Msg {
			return systemAppearanceCheckMsg{}
		}))
	}
	if sv := m.config.GetStartView(); sv != "dashboard" {
		vt := startViewTypeFromString(sv)
		cmds = append(cmds, func() tea.Msg {
			return NavigateToViewMsg{ViewType: vt}
		})
	}
	return tea.Batch(cmds...)
}

// startViewTypeFromString maps a start view config string to a ViewType.
func startViewTypeFromString(sv string) ViewType {
	switch sv {
	case "focus-board":
		return ViewFocusBoard
	case "cross-project-outliner":
		return ViewCrossProjectOutliner
	case "all-tasks":
		return ViewAllTasks
	case "daily-review":
		return ViewDailyReview
	case "backlog":
		return ViewFocusView
	case "priority":
		return ViewPriorityView
	case "due-dates":
		return ViewDueDateOverview
	default:
		return ViewDashboard
	}
}
