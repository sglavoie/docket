package tui

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// planningReconcileEnv is a filesystem-backed workspace wiring the real
// transactional commit service (with the workspace undo ledger), a workspace
// catalog, and a seeded planning domain, so a transition committed through the
// application service folds its planning reconciliation into the same ledger
// transaction exactly as production does.
type planningReconcileEnv struct {
	dataDir       string
	ledger        *storage.WorkspaceLedger
	commitService *storage.CommitService
	repo          *scenarioRepo
	manager       *config.Manager
	runtime       *config.RuntimeState
	service       *app.Service
	project       *models.Project
	taskID        string
}

func newPlanningReconcileEnv(t *testing.T, state models.WorkflowState) *planningReconcileEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitService := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), "planning-reconcile", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Reconcile target")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = state
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.EnsureDefaultSections()
	project.BuildTaskTree()

	repo := newScenarioRepo(commitService, project)
	repo.seed(t)
	seedNoteMigrationCatalog(t, dataDir, project)

	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	service := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil).
		WithPlanningArtifacts(planningReconcileArtifactBuilder(manager))

	return &planningReconcileEnv{
		dataDir:       dataDir,
		ledger:        ledger,
		commitService: commitService,
		repo:          repo,
		manager:       manager,
		runtime:       runtime,
		service:       service,
		project:       project,
		taskID:        task.ID,
	}
}

// seedDailyPlan commits a Daily Plan entry for the env's task through a checked
// planning commit, establishing the Today horizon as a precondition.
func (e *planningReconcileEnv) seedDailyPlan(t *testing.T) {
	t.Helper()
	planning := clonePlanningState(e.runtime.Planning())
	planning.DailyPlan = []config.DailyPlanItem{{TaskID: e.taskID, ProjectID: e.project.ID, PlannedDate: "2026-07-19"}}
	if err := e.runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("seed daily plan: %v", err)
	}
}

// seedFocusQueue commits a Next order entry for the env's task.
func (e *planningReconcileEnv) seedFocusQueue(t *testing.T) {
	t.Helper()
	planning := clonePlanningState(e.runtime.Planning())
	planning.FocusQueue = []config.FocusQueueItem{{TaskID: e.taskID, ProjectID: e.project.ID}}
	if err := e.runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("seed focus queue: %v", err)
	}
}

// reloadPlanning reloads the planning domain from disk so assertions observe the
// committed state.
func (e *planningReconcileEnv) reloadPlanning(t *testing.T) *config.PlanningState {
	t.Helper()
	if err := e.runtime.RefreshAfterCommit(); err != nil {
		t.Fatalf("refresh runtime: %v", err)
	}
	return e.runtime.Planning()
}

// TestPlanningBearingBatchRefreshesInMemoryHorizons locks FR-014-002: a
// workflow-aware coordinator can commit a checked planning artifact while the
// Planner is already open, and the shell must advance every planning horizon
// before refreshing that registered view. The maps are deliberately retained as
// aliases because Planner models hold them directly rather than through pointers.
func TestPlanningBearingBatchRefreshesInMemoryHorizons(t *testing.T) {
	env := newPlanningReconcileEnv(t, models.StateReady)
	env.seedDailyPlan(t)

	task := env.project.GetTaskByID(env.taskID)
	if task == nil {
		t.Fatal("seeded task missing")
	}
	dailyPlan := []FocusTask{{Task: task, Project: env.project}}
	dailyPlanIDs := map[string]bool{task.ID: true}
	focusQueue := []FocusTask{}
	focusQueueIDs := map[string]bool{}

	shell := &AppModel{
		configManager:        env.manager,
		runtime:              env.runtime,
		projects:             map[string]*models.Project{env.project.ID: env.project},
		persister:            NewProjectService(),
		appService:           env.service,
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		projectLastView:      make(map[string]PresentationID),
		viewStack:            []PresentationID{ViewFocusBoard},
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
		currentProject:       env.project,
		focusQueue:           focusQueue,
		focusQueueIDs:        focusQueueIDs,
		dailyPlan:            dailyPlan,
		dailyPlanIDs:         dailyPlanIDs,
		dailyPlanCarriedIDs:  make(map[string]bool),
	}
	planner := NewFocusBoardModel(
		shell.runtime,
		shell.configManager,
		&shell.dailyPlan,
		&shell.focusQueue,
		focusQueueIDs,
		dailyPlanIDs,
		shell.dailyPlanCarriedIDs,
	)
	shell.views.Register(ViewFocusBoard, planner)

	if got := taskRowCount(planner.todayRows, task.ID); got != 1 {
		t.Fatalf("open Planner Today rows before transition = %d, want 1", got)
	}
	if got := taskRowCount(planner.nextRows, task.ID); got != 0 {
		t.Fatalf("open Planner Next rows before transition = %d, want 0", got)
	}

	// Pass nil as the coordinator's config committer so the per-call planning
	// builder is nil. The service default still reconciles the planning artifact,
	// matching the public workflow-cascade contract installed by the TUI shell.
	cmd := newSecondaryCoordinators(env.service, nil).TaskItem.TransitionWorkflow(
		task.ID,
		models.StateNext,
		app.SingleTaskWorkflowOptions{UndoAction: "Change State"},
		func(app.TaskResult) string { return "Moved to Next" },
	)
	msg := cmd()
	batch, ok := msg.(BatchTransactionResultMsg)
	if !ok {
		if failure, failed := msg.(TransactionErrorMsg); failed {
			t.Fatalf("planning-bearing workflow batch failed: %v", failure.Err)
		}
		t.Fatalf("workflow command result = %T, want BatchTransactionResultMsg", msg)
	}
	shell.handleBatchTransactionResult(batch)

	durable := config.NewManager(filepath.Join(env.dataDir, ".tt.json")).DomainCoordinator()
	durablePlanning, err := durable.LoadPlanning()
	if err != nil {
		t.Fatalf("load durable planning: %v", err)
	}
	if got := len(durablePlanning.GetDailyPlan()); got != 0 {
		t.Fatalf("durable Daily Plan entries = %d, want 0", got)
	}
	if got := len(durablePlanning.GetFocusQueue()); got != 0 {
		t.Fatalf("durable Next order entries = %d, want 0", got)
	}
	if got := len(env.runtime.Planning().GetDailyPlan()); got != 0 {
		t.Fatalf("runtime Daily Plan entries = %d, want 0", got)
	}
	if got := len(env.runtime.Planning().GetFocusQueue()); got != 0 {
		t.Fatalf("runtime Next order entries = %d, want 0", got)
	}
	if shell.dailyPlanIDs[task.ID] || dailyPlanIDs[task.ID] || planner.dailyPlanIDs[task.ID] {
		t.Fatal("task remained in AppModel or aliased Daily Plan membership map")
	}
	if got := taskMembershipCount(shell.dailyPlan, task.ID); got != 0 {
		t.Fatalf("AppModel Daily Plan slice count = %d, want 0", got)
	}
	if got := taskMembershipCount(*planner.dailyPlan, task.ID); got != 0 {
		t.Fatalf("Planner-aliased Daily Plan slice count = %d, want 0", got)
	}
	if shell.focusQueueIDs[task.ID] || focusQueueIDs[task.ID] || planner.queueIDs[task.ID] {
		t.Fatal("task unexpectedly entered AppModel or aliased Next order map")
	}
	if got := taskMembershipCount(shell.focusQueue, task.ID); got != 0 {
		t.Fatalf("AppModel Next order slice count = %d, want 0", got)
	}
	if got := taskMembershipCount(*planner.focusQueue, task.ID); got != 0 {
		t.Fatalf("Planner-aliased Next order slice count = %d, want 0", got)
	}
	if got := taskRowCount(planner.todayRows, task.ID); got != 0 {
		t.Fatalf("open Planner Today rows after transition = %d, want 0", got)
	}
	if got := taskRowCount(planner.nextRows, task.ID); got != 1 {
		t.Fatalf("open Planner Next rows after transition = %d, want 1", got)
	}

	// A behavioral identity check catches map replacement even when a rebuilt map
	// is empty: writes through the AppModel fields must remain visible to the
	// aliases retained by the already-open Planner.
	const aliasProbe = "map-alias-probe"
	shell.dailyPlanIDs[aliasProbe] = true
	shell.focusQueueIDs[aliasProbe] = true
	if !dailyPlanIDs[aliasProbe] || !focusQueueIDs[aliasProbe] ||
		!planner.dailyPlanIDs[aliasProbe] || !planner.queueIDs[aliasProbe] {
		t.Fatal("planning membership maps were replaced instead of repopulated in place")
	}
	delete(shell.dailyPlanIDs, aliasProbe)
	delete(shell.focusQueueIDs, aliasProbe)
}

func taskRowCount(rows []boardRow, taskID string) int {
	count := 0
	for _, row := range rows {
		if row.Task != nil && row.Task.ID == taskID {
			count++
		}
	}
	return count
}

// TestTUIStateTransitionsPreservePlanningExclusivity locks FR-009-002 for the TUI
// state-change surfaces: a Today task transitioned to Next through the view-local
// coordinator picker, the global (plain Transition) state picker, or the bulk
// pipeline is dropped from the Daily Plan as part of the same transaction — no
// surface leaves a task in both horizons.
func TestTUIStateTransitionsPreservePlanningExclusivity(t *testing.T) {
	t.Run("view-local coordinator picker drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t, models.StateReady)
		env.seedDailyPlan(t)

		coords := newSecondaryCoordinators(env.service, env.manager)
		cmd := coords.TaskItem.TransitionWorkflow(env.taskID, models.StateNext,
			app.SingleTaskWorkflowOptions{UndoAction: "Change State"},
			func(app.TaskResult) string { return "" })
		assertNotTransactionError(t, cmd())

		planning := env.reloadPlanning(t)
		if len(planning.GetDailyPlan()) != 0 {
			t.Fatalf("daily plan after coordinator transition = %+v, want empty", planning.GetDailyPlan())
		}
	})

	t.Run("global plain-Transition picker drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t, models.StateReady)
		env.seedDailyPlan(t)

		if _, err := env.service.Transition(app.StateRequest{TaskID: env.taskID, Target: models.StateNext}); err != nil {
			t.Fatalf("Transition: %v", err)
		}

		planning := env.reloadPlanning(t)
		if len(planning.GetDailyPlan()) != 0 {
			t.Fatalf("daily plan after global transition = %+v, want empty", planning.GetDailyPlan())
		}
	})

	t.Run("bulk transition drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t, models.StateReady)
		env.seedDailyPlan(t)

		coords := newSecondaryCoordinators(env.service, env.manager)
		cmd := coords.Bulk.TransitionWithWorkflow(
			app.BulkTransitionRequest{TaskIDs: []string{env.taskID}, Target: models.StateNext, SkipInvalid: true},
			func(int) string { return "" })
		assertNotTransactionError(t, cmd())

		planning := env.reloadPlanning(t)
		if len(planning.GetDailyPlan()) != 0 {
			t.Fatalf("daily plan after bulk transition = %+v, want empty", planning.GetDailyPlan())
		}
	})

	t.Run("transition out of Next drops stale order entry", func(t *testing.T) {
		env := newPlanningReconcileEnv(t, models.StateNext)
		env.seedFocusQueue(t)

		if _, err := env.service.Transition(app.StateRequest{TaskID: env.taskID, Target: models.StateInProgress}); err != nil {
			t.Fatalf("Transition: %v", err)
		}

		planning := env.reloadPlanning(t)
		if len(planning.GetFocusQueue()) != 0 {
			t.Fatalf("focus queue after leaving Next = %+v, want empty (stale order dropped)", planning.GetFocusQueue())
		}
	})
}

// TestPlanningToggleUndoOrderingAndDurability locks FR-009-001 for the TUI Next
// queue toggle: q folds the workflow-state change, the Today drop, and the Next
// order entry into ONE workspace-ledger transaction, so (1) an immediate u/ctrl+r
// restores and replays the exact pre-mutation state, (2) the move survives a
// simulated restart (a fresh ledger over the same data dir can still undo it), and
// (3) when a newer workspace-ledger mutation follows the toggle, that newer
// mutation is undone first — the planning undo never jumps ahead of it.
func TestPlanningToggleUndoOrderingAndDurability(t *testing.T) {
	t.Run("q toggle commits state and planning as one durable undoable ledger entry", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewOutliner)
		// Precondition: the task is planned for Today, so the promotion must also
		// prove the Today drop rides the same transaction.
		if err := commitPlanningToRuntime(app.runtime, func(p *config.PlanningState) {
			p.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: app.currentProject.ID, PlannedDate: "2026-07-19"}})
		}); err != nil {
			t.Fatalf("seed daily plan: %v", err)
		}

		app = sendAppKey(app, syntheticKey('q'))
		assertQueueKeyFlow(t, app, task.ID, true)

		// The move is one ledger transaction that captured state + planning.
		dataDir := app.runtime.DataDir()
		if got := diskFocusQueueLen(t, dataDir); got != 1 {
			t.Fatalf("committed Next order entries = %d, want 1", got)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("committed Daily Plan entries = %d, want 0 (Today dropped in same txn)", got)
		}

		// Durability: a fresh ledger over the same data dir (simulating a restart)
		// undoes the transaction and restores the exact pre-mutation state.
		freshLedger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
		if _, _, err := freshLedger.UndoWithEntry(storage.GlobalScope()); err != nil {
			t.Fatalf("durable undo from fresh ledger: %v", err)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after durable undo = %d, want 0", got)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after durable undo = %d, want 1 (Today restored)", got)
		}
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateReady {
			t.Fatalf("task state after durable undo = %s, want ready", st)
		}
	})

	t.Run("newer ledger mutation is undone before the planning toggle", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewOutliner)

		origPriority := diskTaskPriority(t, app.currentProject, task.ID)
		newPriority := origPriority + 1

		// Promote to Next (ledger transaction #1).
		app = sendAppKey(app, syntheticKey('q'))
		assertQueueKeyFlow(t, app, task.ID, true)

		// A newer, unrelated workspace-ledger mutation (transaction #2).
		if _, err := app.appService.SetPriority(task.ID, newPriority); err != nil {
			t.Fatalf("newer mutation (set priority): %v", err)
		}
		app.reloadProjectByID(app.currentProject.ID)

		// u undoes the NEWEST transaction first: the priority change, not the toggle.
		app = sendAppKey(app, syntheticKey('u'))
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateNext {
			t.Fatalf("state after first undo = %s, want still next (priority undone first)", st)
		}
		if got := diskFocusQueueLen(t, app.runtime.DataDir()); got != 1 {
			t.Fatalf("Next order after first undo = %d, want 1 (toggle not yet undone)", got)
		}
		if p := diskTaskPriority(t, app.currentProject, task.ID); p != origPriority {
			t.Fatalf("priority after first undo = %d, want %d (newer priority mutation undone first)", p, origPriority)
		}

		// A second u then undoes the planning toggle, restoring the pre-toggle state.
		app = sendAppKey(app, syntheticKey('u'))
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateReady {
			t.Fatalf("state after second undo = %s, want ready (toggle undone)", st)
		}
		if got := diskFocusQueueLen(t, app.runtime.DataDir()); got != 0 {
			t.Fatalf("Next order after second undo = %d, want 0 (toggle undone)", got)
		}
	})
}

// diskFocusQueueLen / diskDailyPlanLen / diskTaskState read committed state fresh
// from disk so assertions observe exactly what a restart would load.
func diskFocusQueueLen(t *testing.T, dataDir string) int {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	return len(planning.GetFocusQueue())
}

func diskDailyPlanLen(t *testing.T, dataDir string) int {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	return len(planning.GetDailyPlan())
}

func diskTaskState(t *testing.T, project *models.Project, taskID string) models.WorkflowState {
	t.Helper()
	return diskTask(t, project, taskID).State()
}

func diskTaskPriority(t *testing.T, project *models.Project, taskID string) int {
	t.Helper()
	return diskTask(t, project, taskID).Priority
}

func diskTask(t *testing.T, project *models.Project, taskID string) *models.Task {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	task := fresh.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s missing after reload", taskID)
	}
	return task
}

// assertNotTransactionError fails when a coordinator command resolves to an error
// message rather than a successful transaction result.
func assertNotTransactionError(t *testing.T, msg any) {
	t.Helper()
	switch m := msg.(type) {
	case TransactionErrorMsg:
		t.Fatalf("transaction error: %v", m.Err)
	case ErrorMsg:
		t.Fatalf("error: %v", m.Err)
	}
}
