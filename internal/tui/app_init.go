package tui

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/appearance"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
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
		configManager:           manager,
		runtime:                 runtime,
		projects:                make(map[string]*models.Project),
		appearanceDetector:      appearance.NewDetector(),
		persister:               NewProjectService(),
		viewStack:               []ViewType{ViewDashboard},
		sessionStats:            sessionStats{sessionStart: time.Now()},
		width:                   80,
		height:                  24,
		views:                   NewViewRegistry(),
		commandRegistry:         NewCommandRegistry(),
		spinner:                 NewSpinner(),
		navHistory:              NewNavHistory(),
		lastProjectView:         defaultProjectViewType(runtime.Settings()),
		projectLastView:         make(map[string]ViewType),
		outlinerToggleStates:    make(map[string]*OutlinerToggleState),
		priorityViewFilterCache: make(map[string]*PriorityViewCache),
		kanbanViewCache:         make(map[string]*KanbanViewCache),
		focusQueue:              []FocusTask{},
		focusQueueIDs:           make(map[string]bool),
		focusDismissedIDs:       make(map[string]bool),
		dailyPlan:               []FocusTask{},
		dailyPlanIDs:            make(map[string]bool),
		dailyPlanCarriedIDs:     make(map[string]bool),
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
	// Fold Today/Next exclusivity into every workflow-state transition committed
	// through this service (global state pickers, palette actions, pinned/quick-
	// switch state changes), so exclusivity holds regardless of the initiating
	// adapter, matching the coordinator-driven view-local pickers.
	appService.WithPlanningArtifacts(planningReconcileArtifactBuilder(manager))
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
	// Migrate legacy (pre-state-driven) Focus Queue members to StateNext BEFORE
	// hydration prunes non-StateNext entries, so first launch converts every
	// eligible legacy member instead of silently discarding the owner's shortlist.
	// The migration commits through a bare application service (no default planning
	// builder) rooted at the same ledger, so the backfill is one journaled, undoable
	// workspace transaction rather than raw per-project saves.
	migrationErr := app.migrateLegacyFocusQueue(newTUIAppService(dataDir, runtimeDeps, ledger), activeProjects, startupLoads)
	if migrationErr != nil {
		logging.Warn("failed to migrate legacy focus queue: %v", migrationErr)
	}
	app.hydrateFocusQueue(activeProjects, startupLoads, migrationErr == nil)
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

// migrateLegacyFocusQueue backfills StateNext onto Focus Queue members that
// predate the state-driven Next invariant, running before hydrateFocusQueue's
// prune interprets them as stale. It loads each referenced project's task tree,
// then commits the StateNext backfill for every non-terminal, not-yet-StateNext
// member through the application service's BulkTransition — so every changed
// project lands in ONE revision-checked workspace transaction with a durable
// ledger entry, the same journaled and undoable boundary as any startup mutation,
// instead of raw per-project saves. The batch is all-or-nothing: on a commit
// failure nothing is written, so every project state and the FocusQueue index stay
// at their pre-migration values. The service is bare (no default planning-artifact
// builder), keeping the migration a pure state backfill that matches `tt doctor`'s
// repair semantics. Every referenced project must load before the transaction and
// every touched project must reload afterward; any failure is returned so startup
// can preserve the durable legacy order index for a later retry.
func (m *AppModel) migrateLegacyFocusQueue(svc *app.Service, activeProjects []*models.Project, startupLoads map[string]error) error {
	queue := m.runtime.Planning().GetFocusQueue()
	if len(queue) == 0 {
		return nil
	}
	projectsByID := make(map[string]*models.Project, len(activeProjects))
	for _, p := range activeProjects {
		if p != nil {
			projectsByID[p.ID] = p
		}
	}
	loadedProjects := make(map[string]bool, len(queue))
	var loadErrs []error
	for _, item := range queue {
		p := projectsByID[item.ProjectID]
		if p == nil || loadedProjects[p.ID] {
			continue
		}
		loadedProjects[p.ID] = true
		if err := m.loadStartupProject(p, startupLoads); err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("load queue project %s: %w", p.Name, err))
		}
	}
	if err := errors.Join(loadErrs...); err != nil {
		return err
	}
	ids := focus.LegacyNextQueueMemberIDs(queue, activeProjects)
	if len(ids) == 0 {
		return nil
	}
	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskIDs:    ids,
		Target:     models.StateNext,
		UndoAction: "Migrate legacy Next queue",
	})
	if err != nil {
		return err
	}
	touched := make(map[string]bool, len(res.TouchedProjectIDs))
	for _, id := range res.TouchedProjectIDs {
		touched[id] = true
	}
	var reloadErrs []error
	for _, p := range activeProjects {
		if p == nil || !touched[p.ID] {
			continue
		}
		if err := m.persister.Load(p); err != nil {
			reloadErrs = append(reloadErrs, fmt.Errorf("reload migrated project %s: %w", p.Name, err))
		}
	}
	return errors.Join(reloadErrs...)
}

// hydrateFocusQueue rebuilds the in-memory Next order index from the planning
// snapshot. Queue membership is now the StateNext workflow state, so it keeps a
// member only while its task is still StateNext and drops entries whose task is
// missing, terminal, or has left StateNext (prune-on-load, mirroring the
// applyQueueFilter prune). Legacy members that predate the state-driven queue are
// backfilled to StateNext by the `tt doctor` migration, which runs before the
// invariant is enforced on first launch. When migration could not be fully
// confirmed, hydration still exposes cached StateNext tasks but must not rewrite
// the durable index: an unconfirmed entry may be legacy rather than stale.
func (app *AppModel) hydrateFocusQueue(activeProjects []*models.Project, startupLoads map[string]error, allowPrune bool) {
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
			if task := p.GetTaskByID(item.TaskID); task != nil && task.IsInState(models.StateNext) {
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
	if allowPrune && queueChanged {
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
		if m.outliner == nil {
			return nil
		}
		if m.outliner.multiSelectMode && len(m.outliner.selectedTasks) > 0 {
			selected := m.outliner.getSelectedTasks()
			taskIDs := make([]string, 0, len(selected))
			for _, task := range selected {
				taskIDs = append(taskIDs, task.ID)
				m.outliner.project.MutateTask(task.ID, func(t *models.Task) {
					t.Priority = priority
				})
			}
			m.outliner.rebuildRows()
			return bulkTaskPropertyChangedWithUndo("Set Priority", fmt.Sprintf("P%d: %d tasks", priority, len(taskIDs)), taskIDs)
		}
		if task := m.outliner.getCursorTask(); task != nil {
			m.outliner.project.MutateTask(task.ID, func(t *models.Task) {
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
		m.notesCommands(),
		m.otherCommands(),
		m.structureCommands(),
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
	if msg, ok := startupNavigationMsg(m.runtime.Machine().GetStartView()); ok {
		cmds = append(cmds, func() tea.Msg {
			return msg
		})
	}
	return tea.Batch(cmds...)
}

// startupNavigationMsg resolves a start-view config string to the concrete
// navigation message AppModel.Init should dispatch. Aliases that fold onto a
// shared view with a specific subview carry that subview directly so the
// persisted meaning survives startup: "backlog" opens the Planner Backlog tab
// (ShowFocusBoardMsg.SetTab) and "due-dates" opens All Tasks grouped by due
// date (ShowAllTasksMsg.SetGroup), suppressing each view's auto-default. Every
// other start view routes through NavigateToViewMsg. Returns false for
// "dashboard" (already at the bottom of the boot stack) so Init issues no
// redundant push.
func startupNavigationMsg(sv string) (tea.Msg, bool) {
	vt, ok := resolveStartupNavigation(sv)
	if !ok {
		return nil, false
	}
	switch sv {
	case "backlog":
		return ShowFocusBoardMsg{Tab: boardTabBacklog, SetTab: true}, true
	case "due-dates":
		return ShowAllTasksMsg{Group: ATGroupByDueDate, SetGroup: true}, true
	default:
		return NavigateToViewMsg{ViewType: vt}, true
	}
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
		// The standalone Focus View is retired; the "backlog" start-view now
		// lands on the Planner (Focus Board). startupNavigationMsg opens it on
		// the Backlog tab so the persisted meaning survives.
		return ViewFocusBoard
	case "priority":
		return ViewPriorityView
	case "due-dates":
		// The standalone Due Date Overview is retired; the "due-dates"
		// start-view now lands on All Tasks. startupNavigationMsg opens it
		// grouped by due date so the persisted meaning survives.
		return ViewAllTasks
	default:
		return ViewDashboard
	}
}
