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
		configManager:        manager,
		runtime:              runtime,
		projects:             make(map[string]*models.Project),
		appearanceDetector:   appearance.NewDetector(),
		persister:            NewProjectService(),
		navigation:           NewNavigationState(),
		sessionStats:         sessionStats{sessionStart: time.Now()},
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
		operationRegistry:    NewOperationRegistry(),
		spinner:              NewSpinner(),
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		focusQueue:           []FocusTask{},
		focusQueueIDs:        make(map[TaskIdentity]bool),
		focusDismissedIDs:    make(map[string]bool),
		dailyPlan:            []FocusTask{},
		dailyPlanIDs:         make(map[TaskIdentity]bool),
		dailyPlanCarriedIDs:  make(map[TaskIdentity]bool),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)

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

	if ensureErr := app.ensureSystemInboxProject(); ensureErr != nil {
		logging.Warn("failed to ensure system inbox project: %v", ensureErr)
	}
	app.syncProjectCache()
	if ensureErr := app.ensureSandboxProject(); ensureErr != nil {
		logging.Warn("failed to ensure sandbox project: %v", ensureErr)
	}
	app.syncProjectCache()
	if ensureErr := app.ensureProjectColorSlots(); ensureErr != nil {
		logging.Warn("failed to persist project color slots: %v", ensureErr)
	}
	app.syncProjectCache()
	if repairErr := app.reconcileStartupMachineReferences(); repairErr != nil {
		logging.Warn("failed to reconcile startup machine-state references; cleanup will retry: %v", repairErr)
	}
	activeProjects := app.activeProjects()
	startupLoads := make(map[string]error, len(activeProjects))
	// The catalog owns project metadata only. Rehydrate the selected project's
	// storage-owned task tree into the session cache before constructing views;
	// RuntimeState.CurrentProject returns a fresh metadata snapshot and must not
	// become the long-lived shell pointer.
	if currentID := runtime.Machine().CurrentProject; currentID != "" {
		if current := app.projectByID(currentID); current != nil {
			if loadErr := app.loadStartupProject(current, startupLoads); loadErr != nil {
				logging.Warn("failed to load current project %s: %v", current.Name, loadErr)
			}
			app.currentProject = current
		}
	}
	// The Outliner always launches in global All, so a restored current
	// project that is no longer a normal work target must not linger as the shell
	// pointer; drop it before resolving startup navigation.
	if app.currentProject != nil &&
		(app.currentProject.Archived || app.currentProject.Sandbox || app.currentProject.Internal) {
		app.currentProject = nil
	}
	projectID := ""
	if app.currentProject != nil {
		projectID = app.currentProject.ID
	}
	if startup, ok := resolveStartupNavigation(runtime.Machine(), projectID); ok {
		app.startupMessage = startup.Message
		if startup.HasRoute {
			route := startup.Route
			app.startupRoute = &route
			app.startupView = startup.Presentation
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
	} else if changed, reconcileErr := app.appService.ReconcilePlanning(); reconcileErr != nil {
		logging.Warn("failed to reconcile startup planning state: %v", reconcileErr)
	} else if changed {
		app.refreshRuntime()
	}
	app.hydrateFocusQueue(activeProjects, startupLoads, migrationErr == nil)
	app.hydrateDailyPlan(activeProjects, startupLoads, migrationErr == nil)
	app.hydrateRecentVisits(activeProjects, startupLoads)

	app.reviewHub = NewReviewHubModel(app.runtime, app.configManager, app.wireSecondaryCoordinators)
	app.reviewHub.SetPlanningIndicators(app.focusQueueIDs, app.dailyPlanIDs)
	app.reviewHub.SetSize(app.width, app.height-2)
	app.ensureManageHub()

	// Hydrate dismissed IDs from the planning-domain snapshot.
	for _, id := range app.runtime.Planning().GetFocusDismissedIDs() {
		app.focusDismissedIDs[id] = true
	}

	if err := app.registerCoreInputOperations(); err != nil {
		return nil, fmt.Errorf("register core input operations: %w", err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		return nil, fmt.Errorf("register task action operations: %w", err)
	}
	if err := app.registerScopeHubOperations(); err != nil {
		return nil, fmt.Errorf("register scope hub operations: %w", err)
	}
	if err := app.registerHarnessOperations(); err != nil {
		return nil, fmt.Errorf("register harness operations: %w", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		return nil, fmt.Errorf("register simplified input strategy: %w", err)
	}

	ApplyTheme(NewThemeConfig(runtime.Settings()))

	app.openLoops = ComputeOpenLoops(app.runtime, app.dailyPlanIDs)
	app.refreshOnboardingProgress(startupLoads)

	// The Outliner is the sole root: construct and register the global
	// (All Projects) Outliner in All so the first frame renders the root even
	// before Init's startup reload lands. NewNavigationState already roots the
	// primary stack at the Outliner. The returned constructor command is
	// deliberately startup-owned: AppModel.Init
	// emits ShowGlobalOutlinerMsg, whose handler starts and returns the current
	// generation's reload command.
	_ = app.prepareCrossProjectOutliner()
	app.crossOutliner.ApplyOpenOptions(OutlinerOpenOptions{Mode: ModeAll, SetMode: true})
	app.setAccentOnView(app.crossOutliner)

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
	locators := focus.LegacyNextQueueMemberLocators(queue, activeProjects)
	if len(locators) == 0 {
		return nil
	}
	res, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskLocators: appTaskLocatorsFromConfig(locators),
		Target:       models.StateNext,
		UndoAction:   "Migrate legacy Next queue",
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

// hydrateFocusQueue rebuilds the in-memory Next order index from an app-owned
// reconciliation of the planning snapshot. Legacy members that predate the
// state-driven queue are backfilled before this runs. When that migration could
// not be confirmed, hydration exposes only cached StateNext tasks but preserves
// the durable index for a later retry.
func (m *AppModel) hydrateFocusQueue(activeProjects []*models.Project, startupLoads map[string]error, allowReconcile bool) {
	planning := m.runtime.Planning()
	if allowReconcile {
		projectsByID := make(map[string]*models.Project, len(activeProjects))
		for _, project := range activeProjects {
			if project != nil {
				projectsByID[project.ID] = project
			}
		}
		referencedProjects := make(map[string]bool)
		for _, item := range planning.GetFocusQueue() {
			referencedProjects[item.ProjectID] = true
		}
		for _, item := range planning.GetDailyPlan() {
			referencedProjects[item.ProjectID] = true
		}
		var targets []app.PlanningReconcileTarget
		canReconcile := true
		for projectID := range referencedProjects {
			project := projectsByID[projectID]
			if project == nil {
				continue
			}
			if err := m.loadStartupProject(project, startupLoads); err != nil {
				canReconcile = false
				continue
			}
			for _, task := range project.Tasks {
				if task != nil {
					targets = append(targets, app.PlanningReconcileTarget{
						TaskID: task.ID, ProjectID: project.ID, State: task.State(),
					})
				}
			}
		}
		if canReconcile {
			reconciled := clonePlanningState(planning)
			if app.ReconcilePlanningState(reconciled, app.PlanningReconcileRequest{
				Targets: targets, Complete: true,
			}) {
				if m.runtime.DataDir() == "" {
					planning = reconciled
				} else if err := m.runtime.CommitPlanning(reconciled); err != nil {
					logging.Warn("failed to persist reconciled focus queue: %v", err)
				} else {
					planning = m.runtime.Planning()
				}
			}
		}
	}

	for _, item := range planning.GetFocusQueue() {
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := m.loadStartupProject(p, startupLoads); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil && task.IsInState(models.StateNext) {
				m.focusQueue = append(m.focusQueue, FocusTask{Task: task, Project: p})
				m.focusQueueIDs[taskIdentityFor(task, p)] = true
			}
			break
		}
	}
}

// hydrateDailyPlan restores the daily plan from the planning-domain snapshot with
// carry-over logic, committing the carried/pruned plan through a checked planning
// commit.
func (m *AppModel) hydrateDailyPlan(activeProjects []*models.Project, startupLoads map[string]error, reconcile ...bool) {
	allowReconcile := len(reconcile) == 0 || reconcile[0]
	today := time.Now().Format("2006-01-02")
	var updates []app.PlanningTodayUpdate
	planChanged := false
	for _, item := range m.runtime.Planning().GetDailyPlan() {
		var foundTask *models.Task
		var foundProject *models.Project
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := m.loadStartupProject(p, startupLoads); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				foundTask = task
				foundProject = p
			}
			break
		}
		if foundTask == nil || foundProject == nil || !app.PlanningTodayEligible(foundTask.State()) {
			planChanged = true
			continue
		}
		if item.PlannedDate < today {
			if foundTask.IsComplete() {
				planChanged = true
				continue
			}
			item.PlannedDate = today
			m.dailyPlanCarried++
			m.dailyPlanCarriedIDs[taskIdentityFor(foundTask, foundProject)] = true
			planChanged = true
		}
		updates = append(updates, app.PlanningTodayUpdate{
			Locator:     app.TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID},
			PlannedDate: item.PlannedDate,
		})
		m.dailyPlan = append(m.dailyPlan, FocusTask{Task: foundTask, Project: foundProject})
		m.dailyPlanIDs[taskIdentityFor(foundTask, foundProject)] = true
	}
	if planChanged && allowReconcile && m.appService != nil {
		if err := m.appService.UpdateTodayDates(updates); err != nil {
			logging.Warn("failed to persist daily plan carry-over: %v", err)
		} else {
			m.refreshRuntime()
		}
	}
}

// hydrateRecentVisits restores recent task visits from machine-local state,
// pruning stale entries through a checked machine commit.
func (app *AppModel) hydrateRecentVisits(activeProjects []*models.Project, startupLoads map[string]error) {
	visitsChanged := false
	var updatedVisits []config.RecentTaskVisit
	var staleLocators []config.ProjectTaskLocator
	for _, item := range app.runtime.Machine().GetRecentTaskVisits() {
		found := false
		uncertain := false
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.loadStartupProject(p, startupLoads); err != nil {
				uncertain = true
				updatedVisits = append(updatedVisits, item)
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				app.recentTaskVisits = append(app.recentTaskVisits, RecentTaskVisitEntry{Task: task, Project: p, Pinned: item.Pinned})
				updatedVisits = append(updatedVisits, item)
				found = true
			}
			break
		}
		if !found && !uncertain {
			visitsChanged = true
			staleLocators = append(staleLocators, config.ProjectTaskLocator{
				ProjectID: item.ProjectID,
				TaskID:    item.TaskID,
			})
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
			app.retainMachineDomainRepair(machineRepairForArchivedTasks(staleLocators))
		}
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
	if repairCmd := m.rescheduleMachineDomainRepair(); repairCmd != nil {
		cmds = append(cmds, repairCmd)
	}
	if m.runtime.Settings().GetAutoTheme() {
		cmds = append(cmds, tea.Tick(appearanceCheckInterval, func(time.Time) tea.Msg {
			return systemAppearanceCheckMsg{}
		}))
	}
	if m.startupMessage != nil {
		msg := m.startupMessage
		cmds = append(cmds, func() tea.Msg {
			return msg
		})
	}
	return tea.Batch(cmds...)
}

type startupNavigationPlan struct {
	Message      tea.Msg
	Route        Route
	HasRoute     bool
	Presentation PresentationID
}

// resolveStartupNavigation restores only the config-owned Outliner view-state
// DTO. The Outliner root route remains installed by typed NavigationState and
// scope/project selection remains outside the DTO.
func resolveStartupNavigation(machine *config.MachineState, _ string) (startupNavigationPlan, bool) {
	route := config.NormalizeOutlinerRouteState(config.OutlinerRouteState{})
	if machine != nil {
		route = config.NormalizeOutlinerRouteState(machine.StartupRoute)
	}
	return startupNavigationPlan{
		Message: ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{
			Mode:          OutlinerModeID(route.Mode),
			SetMode:       true,
			RouteState:    route,
			SetRouteState: true,
		}},
	}, true
}
