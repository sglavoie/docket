package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// NewAppModel builds the shell over an opened runtime. Every durable piece —
// the domain snapshot, the manager, the ledger, and the application service —
// is the runtime's; the shell adds only presentation. A launch passes the
// runtime engine.Open returned, and a test the one engine.Attach composed over
// a snapshot it built, so both run through one composition.
func NewAppModel(rt *engine.Runtime, opts ...AppOption) (*AppModel, error) {
	runtime, manager := rt.State(), rt.Manager()

	app := &AppModel{
		domain:       newDomainController(runtime, manager),
		viewNav:      newViewNavigationController(),
		operations:   operationhost.NewController(),
		nextOrder:    []outliner.PlanningTask{},
		nextOrderIDs: make(map[taskref.TaskIdentity]bool),
		today:        []outliner.PlanningTask{},
		todayIDs:     make(map[taskref.TaskIdentity]bool),
	}

	// The machine-local advisory setting is read before the options run, so
	// WithHiddenAdvisories (the `--no-advisories` flag) can only ever hide: a
	// per-launch flag suppresses chrome for one run, it never re-enables what the
	// machine file turned off.
	if machine := app.domain.machine(); machine != nil {
		app.feedback.hideAdvisories = machine.AdvisoriesHidden
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}
	app.operations.BindExecutor()

	// Bind the primary feature coordinators to the runtime's application
	// service, so a mutation dispatched from a primary view commits through the
	// same transactional, undoable path as every other frontend. The runtime
	// already folds Today/Next exclusivity into every workflow-state transition
	// the service commits.
	appService := rt.Service()
	app.domain.service = appService
	app.domain.syncStatus = rt.SyncStatus
	app.domain.syncConflicts = rt.SyncConflicts
	app.domain.resolveSyncConflict = rt.ResolveSyncConflict
	app.domain.resolveProjectConflict = rt.ResolveProjectConflict
	app.automaticSync.exchange = rt.BackgroundSyncExchanger(engine.SyncExchangeOptions{AppVersion: AppVersion})
	if app.automaticSync.exchange != nil {
		app.automaticSync.readStatus = backgroundSyncStatusReader(rt.BackgroundRuntime())
		app.automaticSync.ownWrites = rt.OwnWrites()
	}
	app.automaticSync.localSequence = rt.SyncLocalSequenceReader()
	app.features = coordinator.NewFeatureCoordinators(appService)
	app.secondaryFeatures = coordinator.NewSecondaryCoordinators(appService)

	// Complete the domain cluster with the shared history handle the runtime's
	// commit service records into.
	app.domain.ledger = rt.Ledger()

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
	app.lenses.SetPendingLaunch(runtime.Machine().ActiveNamedLens())
	app.lifecycle.startup = launchOutlinerMsg{}
	if changed, reconcileErr := rt.ReconcilePlanning(); reconcileErr != nil {
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

	theme.ApplyTheme(theme.NewThemeConfig(app.domain.machine()))

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
	app.crossOutliner.ApplyOpenOptions(outliner.OutlinerOpenOptions{Mode: chrome.ModeAll, SetMode: true})
	app.setAccentOnView(app.crossOutliner)

	app.automaticSync.stamps = app.workspaceStamps()
	if app.automaticSync.localSequence != nil {
		app.automaticSync.sequence, _ = app.automaticSync.localSequence()
	}
	return app, nil
}

// registerOperationCatalog names the fifteen concrete registrars, in the order
// the catalog is built in, and hands them to the operation controller that owns
// the registries they fill. The order is load-bearing and the table is the whole
// of it: a family is added here or it is not registered at all.
func (app *AppModel) registerOperationCatalog() error {
	return app.operations.RegisterCatalog([]operationhost.NamedRegistrar{
		{Name: "core input", Register: app.registerCoreInputOperations},
		{Name: "task action", Register: app.registerTaskActionOperations},
		{Name: "filter", Register: app.registerFilterOperations},
		{Name: "sort", Register: app.registerSortOperations},
		{Name: "project hub", Register: app.registerProjectHubOperations},
		{Name: "project", Register: app.registerProjectOperations},
		{Name: "project action sheet", Register: app.registerProjectActionSheetOperation},
		{Name: "project bulk", Register: app.registerProjectBulkOperations},
		{Name: "project tag vocabulary", Register: app.registerProjectTagVocabularyOperations},
		{Name: "note", Register: app.registerNoteOperations},
		{Name: "notes surface", Register: app.registerNotesSurfaceOperations},
		{Name: "attachment", Register: app.registerAttachmentOperations},
		{Name: "comment", Register: app.registerCommentOperations},
		{Name: "checklist row", Register: app.registerChecklistRowOperations},
		{Name: "lens", Register: app.registerLensOperations},
	}, surfaceOperationGates())
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
	err := app.domain.persister.Load(project)
	loaded[key] = err
	return err
}

// hydrateNextOrder rebuilds the in-memory Next membership/order from planning.
func (m *AppModel) hydrateNextOrder(workspaceProjects []*models.Project, startupLoads map[string]error, allowReconcile bool) {
	planning := m.domain.runtime.Planning()
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
		var unavailable []string
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
			// No tasks.md loads as an empty project, which is not evidence
			// that the tasks this queue names are gone.
			if storage.ProjectTasksFileMissing(project) {
				unavailable = append(unavailable, project.ID)
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
			reconciled := coordinator.ClonePlanningState(planning)
			if app.ReconcilePlanningState(reconciled, app.PlanningReconcileRequest{
				Targets: targets, Complete: true, Unavailable: unavailable,
			}) {
				if m.domain.runtime.DataDir() == "" {
					planning = reconciled
				} else if err := m.domain.runtime.CommitPlanning(reconciled); err != nil {
					logging.Warn("failed to persist reconciled Next order: %v", err)
				} else {
					planning = m.domain.runtime.Planning()
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
				m.nextOrder = append(m.nextOrder, outliner.PlanningTask{Task: task, Project: p})
				m.nextOrderIDs[taskref.For(task, p)] = true
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
	for _, item := range m.domain.runtime.Planning().GetToday() {
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
		m.today = append(m.today, outliner.PlanningTask{Task: foundTask, Project: foundProject})
		m.todayIDs[taskref.For(foundTask, foundProject)] = true
	}
	if planChanged && allowReconcile && m.domain.service != nil {
		if _, err := m.domain.service.ReconcilePlanning(); err != nil {
			logging.Warn("failed to persist reconciled Today membership: %v", err)
		} else {
			m.refreshRuntime()
		}
	}
}

func (m *AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.lifecycle.periodicSaveCmd(), m.automaticSyncTickCmd(), m.startAutomaticSync()}
	if startupCmd := m.lifecycle.startupCmd(); startupCmd != nil {
		cmds = append(cmds, startupCmd)
	}
	// The note index is loaded off the launch path rather than in NewAppModel:
	// nothing renders it before the first frame, and a synchronous read of every
	// note collection would sit in front of that frame.
	if noteCmd := m.requestNoteSearchIndexLoad(); noteCmd != nil {
		cmds = append(cmds, noteCmd)
	}
	return tea.Batch(cmds...)
}

// serviceNow is the application service's clock, which every committed write
// should be stamped from, or the wall clock where a harness attached no
// service.
func (m *AppModel) serviceNow() time.Time {
	if m.domain.service != nil {
		return m.domain.service.Now()
	}
	return time.Now()
}
