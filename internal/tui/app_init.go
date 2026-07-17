package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/appearance"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

// NewAppModel creates a new TUI application model from the startup-loaded
// configuration domains and the services rooted at the same workspace.
func NewAppModel(runtime *config.RuntimeState, manager *config.Manager, opts ...AppOption) (*AppModel, error) {
	var err error
	runtime, err = recoverTUIWorkspace(runtime)
	if err != nil {
		return nil, err
	}
	dataDir := runtime.DataDir()

	resetSessionMutations()

	app := &AppModel{
		configManager:            manager,
		runtime:                  runtime,
		projects:                 make(map[string]*models.Project),
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
		lastProjectView:          defaultProjectViewType(runtime.Settings()),
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

	// Bind the primary feature coordinators to the application service. The
	// service is built once per session over the storage-backed repository so a
	// mutation dispatched from a primary view commits through the same
	// transactional, undoable path as the equivalent CLI command.
	var runtimeDeps *config.RuntimeDependencies
	if manager != nil {
		runtimeDeps = manager.RuntimeDependencies()
	}
	depth := effectiveUndoHistoryDepth(runtime.Settings())
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), depth, []string{dataDir})
	appService := newTUIAppService(dataDir, runtimeDeps, ledger)
	app.appService = appService
	app.features = newFeatureCoordinators(appService, appService)
	app.secondaryFeatures = newSecondaryCoordinators(appService, manager)

	// Assemble the domain-native dependency bundle shared by the shell and views.
	app.deps = newShellDeps(runtime, appService, manager, ledger)

	if err := app.ensureSystemInboxProject(); err != nil {
		logging.Warn("failed to ensure system inbox project: %v", err)
	}
	app.syncProjectCache()
	if err := app.ensureSandboxProject(); err != nil {
		logging.Warn("failed to ensure sandbox project: %v", err)
	}
	app.syncProjectCache()
	if err := app.ensureProjectColorSlots(); err != nil {
		logging.Warn("failed to persist project color slots: %v", err)
	}
	app.syncProjectCache()
	activeProjects := app.activeProjects()
	startupLoads := make(map[string]error, len(activeProjects))
	// The catalog owns project metadata only. Rehydrate the selected project's
	// storage-owned task tree into the session cache before constructing views;
	// RuntimeState.CurrentProject returns a fresh metadata snapshot and must not
	// become the long-lived shell pointer.
	if currentID := runtime.Machine().CurrentProject; currentID != "" {
		if current := app.projectByID(currentID); current != nil {
			if err := app.loadStartupProject(current, startupLoads); err != nil {
				logging.Warn("failed to load current project %s: %v", current.Name, err)
			}
			app.currentProject = current
		}
	}
	app.hydrateFocusQueue(activeProjects, startupLoads)
	app.hydrateDailyPlan(activeProjects, startupLoads)
	app.hydrateRecentVisits(activeProjects, startupLoads)

	// Hydrate dismissed IDs from the planning-domain snapshot.
	for _, id := range app.runtime.Planning().GetFocusDismissedIDs() {
		app.focusDismissedIDs[id] = true
	}

	// Initialize dashboard and register it
	app.dashboard = NewDashboardModel(runtime)
	app.views.Register(ViewDashboard, app.dashboard)

	// Record initial dashboard view in navigation history
	app.navHistory.Record(NavEntry{ViewType: ViewDashboard})

	// Register global commands
	app.registerGlobalCommands()

	ApplyTheme(NewThemeConfig(runtime.Settings()))
	app.dashboard.SetAccentColor(AccentDashboard)

	app.openLoops = ComputeOpenLoops(app.runtime, app.dailyPlanIDs)
	app.refreshOnboardingProgress(startupLoads)

	return app, nil
}

// recoverTUIWorkspace runs before any startup workflow evaluation, domain
// consumption, or project load. NewAppModel also calls it so direct/headless
// model construction has the same crash-recovery guarantee as RunApp.
func recoverTUIWorkspace(runtime *config.RuntimeState) (*config.RuntimeState, error) {
	if runtime == nil {
		return config.EmptyRuntimeState(""), nil
	}
	dataDir := runtime.DataDir()
	if dataDir == "" {
		return runtime, nil
	}
	report, err := storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir}).RecoverWorkspace()
	if err != nil {
		return nil, fmt.Errorf("recover workspace before TUI startup: %w", err)
	}
	if len(report.RolledForward) == 0 && len(report.CleanedUp) == 0 {
		return runtime, nil
	}
	recovered, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		return nil, fmt.Errorf("reload recovered workspace domains: %w", err)
	}
	return recovered, nil
}

// hydrateFocusQueue restores the focus queue from the planning-domain snapshot,
// pruning stale entries and committing the pruned queue through a checked
// planning commit.
func (app *AppModel) loadStartupProject(project *models.Project, loaded map[string]error) error {
	if project == nil {
		return fmt.Errorf("project is required")
	}
	key := project.ID
	if key == "" {
		key = project.Path
	}
	if err, ok := loaded[key]; ok {
		return err
	}
	err := app.persister.Load(project)
	loaded[key] = err
	return err
}

func (app *AppModel) hydrateFocusQueue(activeProjects []*models.Project, startupLoads map[string]error) {
	queueChanged := false
	var updatedQueue []config.FocusQueueItem
	for _, item := range app.runtime.Planning().GetFocusQueue() {
		found := false
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.loadStartupProject(p, startupLoads); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil && !task.IsComplete() {
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
		if err := commitPlanningToRuntime(app.runtime, func(p *config.PlanningState) {
			p.SetFocusQueue(updatedQueue)
		}); err != nil {
			logging.Warn("failed to persist focus queue pruning: %v", err)
		}
	}
}

// hydrateDailyPlan restores the daily plan from the planning-domain snapshot with
// carry-over logic, committing the carried/pruned plan through a checked planning
// commit.
func (app *AppModel) hydrateDailyPlan(activeProjects []*models.Project, startupLoads map[string]error) {
	today := time.Now().Format("2006-01-02")
	var updatedPlan []config.DailyPlanItem
	planChanged := false
	for _, item := range app.runtime.Planning().GetDailyPlan() {
		var foundTask *models.Task
		var foundProject *models.Project
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.loadStartupProject(p, startupLoads); err != nil {
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
			if foundTask.IsComplete() {
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
		if err := commitPlanningToRuntime(app.runtime, func(p *config.PlanningState) {
			p.SetDailyPlan(updatedPlan)
		}); err != nil {
			logging.Warn("failed to persist daily plan carry-over: %v", err)
		}
	}
}

// hydrateRecentVisits restores recent task visits from machine-local state,
// pruning stale entries through a checked machine commit.
func (app *AppModel) hydrateRecentVisits(activeProjects []*models.Project, startupLoads map[string]error) {
	visitsChanged := false
	var updatedVisits []config.RecentTaskVisit
	for _, item := range app.runtime.Machine().GetRecentTaskVisits() {
		found := false
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.loadStartupProject(p, startupLoads); err != nil {
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
		if err := commitMachineToRuntime(app.runtime, func(machine *config.MachineState) {
			machine.SetRecentTaskVisits(updatedVisits)
		}); err != nil {
			logging.Warn("failed to persist recent task visits pruning: %v", err)
		}
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
		m.focusBoardCommands(),
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
	if m.runtime.Settings().GetAutoTheme() {
		cmds = append(cmds, tea.Tick(appearanceCheckInterval, func(time.Time) tea.Msg {
			return systemAppearanceCheckMsg{}
		}))
	}
	if vt, ok := resolveStartupNavigation(m.runtime.Machine().GetStartView()); ok {
		cmds = append(cmds, func() tea.Msg {
			return NavigateToViewMsg{ViewType: vt}
		})
	}
	return tea.Batch(cmds...)
}

// resolveStartupNavigation decides whether AppModel.Init should push a
// startup view onto the dashboard-rooted stack. It returns the destination
// ViewType and true when navigation is required; "dashboard" (the only
// surface that matches the initial stack) returns false so the user stays on
// the dashboard without a redundant push.
func resolveStartupNavigation(sv string) (ViewType, bool) {
	if sv == "dashboard" {
		return ViewDashboard, false
	}
	return startViewTypeFromString(sv), true
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
