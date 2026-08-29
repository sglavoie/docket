package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// WithPersister sets a custom Persister for the AppModel.
// This is primarily used for testing with mock implementations.
// NewAppModel creates a new TUI application model from the startup-loaded
// configuration domains and the services rooted at the same workspace.
func NewAppModel(runtime *config.RuntimeState, manager *config.Manager, opts ...AppOption) (*AppModel, error) {
	var err error
	runtime, err = recoverTUIWorkspace(runtime)
	if err != nil {
		return nil, err
	}
	dataDir := runtime.DataDir()

	app := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		projects:          make(map[string]*models.Project),
		persister:         NewProjectService(),
		navigation:        NewNavigationState(),
		width:             80,
		height:            24,
		views:             NewViewRegistry(),
		operationRegistry: NewOperationRegistry(),
		nextOrder:         []PlanningTask{},
		nextOrderIDs:      make(map[TaskIdentity]bool),
		today:             []PlanningTask{},
		todayIDs:          make(map[TaskIdentity]bool),
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
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
	appService := newTUIAppService(dataDir, runtimeDeps, ledger)
	// Fold Today/Next exclusivity into every workflow-state transition committed
	// through this service (global state pickers, palette actions, pinned/quick-
	// switch state changes), so exclusivity holds regardless of the initiating
	// adapter, matching the coordinator-driven view-local pickers.
	appService.WithPlanningArtifacts(planningReconcileArtifactBuilder(manager))
	app.appService = appService
	app.features = newFeatureCoordinators(appService)
	app.secondaryFeatures = newSecondaryCoordinators(appService)

	// Assemble the domain-native dependency bundle shared by the shell and views.
	app.deps = newShellDeps(ledger)

	if ensureErr := app.ensureSystemUnfiledProject(); ensureErr != nil {
		logging.Warn("failed to ensure system Unfiled project: %v", ensureErr)
	}
	app.syncProjectCache()
	if ensureErr := app.ensureProjectColorSlots(); ensureErr != nil {
		logging.Warn("failed to persist project color slots: %v", ensureErr)
	}
	app.syncProjectCache()
	workspaceProjects := app.taskWorkspaceProjects()
	startupLoads := make(map[string]error, len(workspaceProjects))
	// The catalog owns project metadata only. Rehydrate the base task workspace's
	// task trees into the session cache before constructing views.
	if app.currentProject != nil && app.currentProject.Archived {
		app.currentProject = nil
	}
	// Launch is deliberately not reconstructed from machine-local navigation
	// state. The first message performs an explicit clean-root reset so every run
	// begins in the same Active Projects / All Outliner regardless of stale v4
	// StartupRoute, CurrentRoute, project-route, search, or selection data. A
	// named lens is the one thing layered on top of that clean start, and it is
	// not navigation: it is a saved set of projection axes the user named and
	// asked to reopen with. The pointer is read here, before any handler runs,
	// because the reset itself clears it — reading it now and draining it after
	// the population lands is what makes the order race-free without teaching the
	// reset about a case it should not know.
	app.pendingLaunchLens = runtime.Machine().ActiveNamedLens()
	app.startupMessage = launchOutlinerMsg{}
	if changed, reconcileErr := app.appService.ReconcilePlanning(); reconcileErr != nil {
		logging.Warn("failed to reconcile startup planning state: %v", reconcileErr)
	} else if changed {
		app.refreshRuntime()
	}
	app.hydrateNextOrder(workspaceProjects, startupLoads, true)
	app.hydrateToday(workspaceProjects, startupLoads, true)

	app.ensureManageHub()

	if err := app.registerOperationCatalog(); err != nil {
		return nil, err
	}

	ApplyTheme(NewThemeConfig(runtime.Machine()))

	app.refreshWorkspacePulseAt(time.Now())
	app.refreshOutstandingConflicts()

	// The Outliner is the sole root: construct and register the global
	// (Active Projects) Outliner in All so the first frame renders the root even
	// before Init's startup reload lands. NewNavigationState already roots the
	// primary stack at the Outliner. The returned constructor command is
	// deliberately startup-owned: AppModel.Init
	// emits launchOutlinerMsg, whose handler resets the root and returns the
	// current generation's reload command.
	_ = app.prepareCrossProjectOutliner()
	app.crossOutliner.ApplyOpenOptions(OutlinerOpenOptions{Mode: ModeAll, SetMode: true})
	app.setAccentOnView(app.crossOutliner)

	return app, nil
}

// registerOperationCatalog fills the registry with every operation family and
// the input strategy that routes their direct keys.
func (app *AppModel) registerOperationCatalog() error {
	registrations := []struct {
		name     string
		register func() error
	}{
		{"core input", app.registerCoreInputOperations},
		{"task action", app.registerTaskActionOperations},
		{"filter", app.registerFilterOperations},
		{"sort", app.registerSortOperations},
		{"project hub", app.registerProjectHubOperations},
		{"project", app.registerProjectOperations},
		{"project action sheet", app.registerProjectActionSheetOperation},
		{"project bulk", app.registerProjectBulkOperations},
		{"project tag vocabulary", app.registerProjectTagVocabularyOperations},
		{"note", app.registerNoteOperations},
		{"notes surface", app.registerNotesSurfaceOperations},
		{"attachment", app.registerAttachmentOperations},
		{"comment", app.registerCommentOperations},
		{"checklist row", app.registerChecklistRowOperations},
		{"lens", app.registerLensOperations},
	}
	for _, registration := range registrations {
		if err := registration.register(); err != nil {
			return fmt.Errorf("register %s operations: %w", registration.name, err)
		}
	}
	return app.registerSurfaceInputStrategies()
}

// registerSurfaceInputStrategies holds every manifested surface to the one-path
// rule and then installs its grammar. A surface that declares no bindings yet
// names only its identity, so it passes through without a strategy.
func (app *AppModel) registerSurfaceInputStrategies() error {
	for _, gate := range surfaceOperationGates() {
		if gate.validate != nil {
			if err := gate.validate(gate.strategy); err != nil {
				return fmt.Errorf("validate direct and menu placement for %s: %w", gate.strategy.ID, err)
			}
		}
		if len(gate.strategy.Bindings) == 0 {
			continue
		}
		if err := app.operationRegistry.RegisterInputStrategy(gate.strategy); err != nil {
			return fmt.Errorf("register %s input strategy: %w", gate.strategy.ID, err)
		}
	}
	return nil
}

// recoverTUIWorkspace runs before domain consumption or project load.
// NewAppModel also calls it so direct/headless
// model construction has the same crash-recovery guarantee as RunApp.
func recoverTUIWorkspace(runtime *config.RuntimeState) (*config.RuntimeState, error) {
	if runtime == nil {
		return config.EmptyRuntimeState(""), nil
	}
	dataDir := runtime.DataDir()
	if dataDir == "" {
		return runtime, nil
	}
	report, err := storage.NewCommitService(
		config.TransactionsDir(dataDir),
		[]string{dataDir, config.LocalDataRoot()},
	).RecoverWorkspace()
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

// hydrateNextOrder restores the focus queue from the planning-domain snapshot,
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

// hydrateNextOrder rebuilds the in-memory Next membership/order from planning.
func (m *AppModel) hydrateNextOrder(workspaceProjects []*models.Project, startupLoads map[string]error, allowReconcile bool) {
	planning := m.runtime.Planning()
	if allowReconcile {
		projectsByID := make(map[string]*models.Project, len(workspaceProjects))
		for _, project := range workspaceProjects {
			if project != nil {
				projectsByID[project.ID] = project
			}
		}
		referencedProjects := make(map[string]bool)
		for _, item := range planning.GetNext() {
			referencedProjects[item.ProjectID] = true
		}
		for _, item := range planning.GetToday() {
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
					logging.Warn("failed to persist reconciled Next order: %v", err)
				} else {
					planning = m.runtime.Planning()
				}
			}
		}
	}

	for _, item := range planning.GetNext() {
		for _, p := range workspaceProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := m.loadStartupProject(p, startupLoads); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil && task.State().IsNonterminal() {
				m.nextOrder = append(m.nextOrder, PlanningTask{Task: task, Project: p})
				m.nextOrderIDs[taskIdentityFor(task, p)] = true
			}
			break
		}
	}
}

// hydrateToday restores the project-qualified Today membership/order. Legacy
// planned dates are intentionally ignored: they cannot confer membership or
// trigger carry-over behavior in the lean runtime model.
func (m *AppModel) hydrateToday(workspaceProjects []*models.Project, startupLoads map[string]error, reconcile ...bool) {
	allowReconcile := len(reconcile) == 0 || reconcile[0]
	planChanged := false
	for _, item := range m.runtime.Planning().GetToday() {
		var foundTask *models.Task
		var foundProject *models.Project
		for _, p := range workspaceProjects {
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
		m.today = append(m.today, PlanningTask{Task: foundTask, Project: foundProject})
		m.todayIDs[taskIdentityFor(foundTask, foundProject)] = true
	}
	if planChanged && allowReconcile && m.appService != nil {
		if _, err := m.appService.ReconcilePlanning(); err != nil {
			logging.Warn("failed to persist reconciled Today membership: %v", err)
		} else {
			m.refreshRuntime()
		}
	}
}

const periodicSaveInterval = 60 * time.Second

func (m *AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
			return periodicSaveMsg{at: at}
		}),
	}
	if m.startupMessage != nil {
		msg := m.startupMessage
		cmds = append(cmds, func() tea.Msg {
			return msg
		})
	}
	// The note index is loaded off the launch path rather than in NewAppModel:
	// nothing renders it before the first frame, and a synchronous read of every
	// note collection would sit in front of that frame.
	if noteCmd := m.requestNoteSearchIndexLoad(); noteCmd != nil {
		cmds = append(cmds, noteCmd)
	}
	return tea.Batch(cmds...)
}
