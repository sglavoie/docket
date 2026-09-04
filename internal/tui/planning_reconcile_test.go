package tui

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
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

//nolint:govet // Independent seed operations use scoped errors before the final runtime load.
func newPlanningReconcileEnv(t *testing.T) *planningReconcileEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitService := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), "planning-reconcile", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	task, err := models.NewGenericTask(uuid.New().String(), 1, "Reconcile target")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateBacklog
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	repo := newScenarioRepo(commitService, project)
	repo.seed(t)
	seedNoteMigrationCatalog(t, dataDir, project)

	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := manager.CommitPlanning(config.NewPlanningState()); err != nil {
		t.Fatalf("seed planning: %v", err)
	}
	if err := manager.CommitMachine(config.NewMachineState()); err != nil {
		t.Fatalf("seed machine: %v", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	service := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir)).
		WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(manager))

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

// seedToday commits a Daily Plan entry for the env's task through a checked
// planning commit, establishing Today membership as a precondition.
func (e *planningReconcileEnv) seedToday(t *testing.T) {
	t.Helper()
	planning := coordinator.ClonePlanningState(e.runtime.Planning())
	planning.Today = []config.ProjectTaskLocator{{TaskID: e.taskID, ProjectID: e.project.ID}}
	if err := e.runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("seed daily plan: %v", err)
	}
}

// seedNextOrder commits a Next order entry for the env's task.
func (e *planningReconcileEnv) seedNextOrder(t *testing.T) {
	t.Helper()
	planning := coordinator.ClonePlanningState(e.runtime.Planning())
	planning.SetNext([]config.ProjectTaskLocator{{TaskID: e.taskID, ProjectID: e.project.ID}})
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

// TestTUIStateTransitionsPreservePlanningExclusivity locks FR-009-002 for the TUI
// state-change surfaces: a Today task moved to another lifecycle state through
// the view-local coordinator picker, the global state picker, or the bulk
// pipeline is dropped from Today as part of the same transaction.
func TestTUIStateTransitionsPreservePlanningExclusivity(t *testing.T) {
	t.Run("view-local coordinator picker drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t)
		env.seedToday(t)

		coords := coordinator.NewSecondaryCoordinators(env.service)
		cmd := coords.TaskItem.Transition(
			taskref.TaskIdentity{ProjectID: env.project.ID, TaskID: env.taskID},
			models.StateWaiting,
		)
		assertNotTransactionError(t, cmd())

		planning := env.reloadPlanning(t)
		if len(planning.GetToday()) != 0 {
			t.Fatalf("daily plan after coordinator transition = %+v, want empty", planning.GetToday())
		}
	})

	t.Run("global plain-Transition picker drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t)
		env.seedToday(t)

		if _, err := env.service.Transition(app.StateRequest{Locator: app.TaskLocator{ProjectID: env.project.ID, TaskID: env.taskID}, Target: models.StateWaiting}); err != nil {
			t.Fatalf("Transition: %v", err)
		}

		planning := env.reloadPlanning(t)
		if len(planning.GetToday()) != 0 {
			t.Fatalf("daily plan after global transition = %+v, want empty", planning.GetToday())
		}
	})

	t.Run("bulk transition drops Today", func(t *testing.T) {
		env := newPlanningReconcileEnv(t)
		env.seedToday(t)

		coords := coordinator.NewSecondaryCoordinators(env.service)
		cmd := coords.Bulk.TransitionLocators(
			[]app.TaskLocator{{ProjectID: env.project.ID, TaskID: env.taskID}},
			models.StateWaiting,
			nil,
		)
		assertNotTransactionError(t, cmd())

		planning := env.reloadPlanning(t)
		if len(planning.GetToday()) != 0 {
			t.Fatalf("daily plan after bulk transition = %+v, want empty", planning.GetToday())
		}
	})

	t.Run("transition out of Next drops stale order entry", func(t *testing.T) {
		env := newPlanningReconcileEnv(t)
		env.seedNextOrder(t)

		if _, err := env.service.Transition(app.StateRequest{Locator: app.TaskLocator{ProjectID: env.project.ID, TaskID: env.taskID}, Target: models.StateDoing}); err != nil {
			t.Fatalf("Transition: %v", err)
		}

		planning := env.reloadPlanning(t)
		if len(planning.GetNext()) != 0 {
			t.Fatalf("focus queue after leaving Next = %+v, want empty (stale order dropped)", planning.GetNext())
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
	t.Run("Next toggle commits state and planning as one durable undoable ledger entry", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)
		// Precondition: the task is planned for Today, so the promotion must also
		// prove the Today drop rides the same transaction.
		if err := commitPlanningToRuntime(app.domain.runtime, func(p *config.PlanningState) {
			p.SetToday([]config.ProjectTaskLocator{{TaskID: task.ID, ProjectID: app.currentProject.ID}})
		}); err != nil {
			t.Fatalf("seed daily plan: %v", err)
		}

		app = sendAppKey(app, syntheticKey('$'))
		app = sendAppKey(app, syntheticKey('n'))
		assertQueueKeyFlow(t, app, task.ID, true)

		// The move is one ledger transaction that captured state + planning.
		dataDir := app.domain.runtime.DataDir()
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("committed Next order entries = %d, want 1", got)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("committed Daily Plan entries = %d, want 0 (Today dropped in same txn)", got)
		}

		// Durability: a fresh ledger over the same data dir (simulating a restart)
		// undoes the transaction and restores the exact pre-mutation state.
		freshLedger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
		if _, _, err := freshLedger.UndoWithEntry(storage.GlobalScope()); err != nil {
			t.Fatalf("durable undo from fresh ledger: %v", err)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after durable undo = %d, want 0", got)
		}
		if got := diskTodayLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after durable undo = %d, want 1 (Today restored)", got)
		}
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("task state after durable undo = %s, want ready", st)
		}
	})

	t.Run("newer ledger mutation is undone before the planning toggle", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)

		origPriority := diskTaskPriority(t, app.currentProject, task.ID)
		newPriority := origPriority + 1

		// Promote to Next (ledger transaction #1).
		app = sendAppKey(app, syntheticKey('$'))
		app = sendAppKey(app, syntheticKey('n'))
		assertQueueKeyFlow(t, app, task.ID, true)

		// A newer, unrelated workspace-ledger mutation (transaction #2).
		if _, err := app.domain.service.SetPriority(taskref.For(task, app.currentProject).Locator(), newPriority); err != nil {
			t.Fatalf("newer mutation (set priority): %v", err)
		}
		app.reloadProjectByID(app.currentProject.ID)

		// u undoes the NEWEST transaction first: the priority change, not the toggle.
		app = sendAppKey(app, syntheticKey('u'))
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("state after first undo = %s, want still next (priority undone first)", st)
		}
		if got := diskNextOrderLen(t, app.domain.runtime.DataDir()); got != 1 {
			t.Fatalf("Next order after first undo = %d, want 1 (toggle not yet undone)", got)
		}
		if p := diskTaskPriority(t, app.currentProject, task.ID); p != origPriority {
			t.Fatalf("priority after first undo = %d, want %d (newer priority mutation undone first)", p, origPriority)
		}

		// A second u then undoes the planning toggle, restoring the pre-toggle state.
		app = sendAppKey(app, syntheticKey('u'))
		if st := diskTaskState(t, app.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("state after second undo = %s, want ready (toggle undone)", st)
		}
		if got := diskNextOrderLen(t, app.domain.runtime.DataDir()); got != 0 {
			t.Fatalf("Next order after second undo = %d, want 0 (toggle undone)", got)
		}
	})
}

// diskNextOrderLen / diskTodayLen / diskTaskState read committed state fresh
// from disk so assertions observe exactly what a restart would load.
func diskNextOrderLen(t *testing.T, dataDir string) int {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	return len(planning.GetNext())
}

func diskTodayLen(t *testing.T, dataDir string) int {
	t.Helper()
	planning, err := config.NewManager(filepath.Join(dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	return len(planning.GetToday())
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
	case coordinator.TransactionErrorMsg:
		t.Fatalf("transaction error: %v", m.Err)
	case status.ErrorMsg:
		t.Fatalf("error: %v", m.Err)
	}
}
