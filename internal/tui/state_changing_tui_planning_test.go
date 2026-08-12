package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TestStateChangingTUISurfacesPreservePlanningExclusivity locks FR-011-002 for
// the live inline editor: lifecycle moves fold Today/Next exclusivity into the
// same durable transaction as the task write.
func TestStateChangingTUISurfacesPreservePlanningExclusivity(t *testing.T) {
	t.Run("atomic inline lifecycle edit drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		parsed, err := parseQuickEdit("Task $waiting", task, m.currentProject)
		if err != nil {
			t.Fatalf("parse inline edit: %v", err)
		}
		cmd := m.secondaryFeatures.TaskItem.EditTaskLine(
			taskLineEditRequest(taskIdentityFor(task, m.currentProject), parsed), nil,
		)
		assertNotTransactionError(t, cmd())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after inline edit = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after cross quick edit into Next = %d, want 0", got)
		}
	})

}

func TestInlineProjectMoveAfterTodayMutationUsesFreshStructuralPlanning(t *testing.T) {
	moving := seedTask(t, 1, "Move me", models.StateBacklog)
	kept := seedTask(t, 1, "Keep disk-only membership", models.StateBacklog)
	env := newCrossProjectTestEnv(t, []*models.Task{moving}, []*models.Task{kept})
	source, destination := env.projects[0], env.projects[1]

	seedNoteMigrationCatalog(t, env.dataDir, source, destination)
	manager := config.NewManager(filepath.Join(env.dataDir, ".tt.json"))
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

	model := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		appService:        env.svc,
		secondaryFeatures: newSecondaryCoordinators(env.svc),
		projects: map[string]*models.Project{
			source.ID:      source,
			destination.ID: destination,
		},
		currentProject: source,
		nextOrder:      []PlanningTask{},
		nextOrderIDs:   make(map[TaskIdentity]bool),
		today:          []PlanningTask{},
		todayIDs:       make(map[TaskIdentity]bool),
	}

	// A live Today mutation installs the runtime-bound reconciliation builder as
	// the service default. The inline project move must not inherit that mutable
	// default after planning.json advances independently of the shell snapshot.
	if _, _, _, err := model.applyTaskPlanningAction(moving.ID, source.ID, app.PlanningAddToday); err != nil {
		t.Fatalf("add moving task to Today: %v", err)
	}
	external := config.NewManager(filepath.Join(env.dataDir, ".tt.json"))
	planning, err := external.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning for external update: %v", err)
	}
	planning.AddToToday(kept.ID, destination.ID)
	if err := external.CommitPlanning(planning); err != nil {
		t.Fatalf("commit external planning update: %v", err)
	}
	if got := len(runtime.Planning().GetToday()); got != 1 {
		t.Fatalf("stale shell Today entries = %d, want 1", got)
	}

	parsed, err := parseQuickEdit(
		moving.Title+" %"+destination.Slug,
		moving,
		source,
		[]*models.Project{source, destination},
	)
	if err != nil {
		t.Fatalf("parse inline project move: %v", err)
	}
	outliner := NewOutlinerModel(nil, manager, runtime)
	req := outliner.taskLineEditRequest(taskIdentityFor(moving, source), parsed)
	if req.PlanningArtifacts == nil {
		t.Fatal("cross-project inline request has no explicit planning builder")
	}
	raw := model.secondaryFeatures.TaskItem.EditTaskLine(req, nil)()
	edited, ok := raw.(TaskLineEditedMsg)
	if !ok {
		if failed, isFailure := raw.(TaskLineEditFailedMsg); isFailure {
			t.Fatalf("inline project move failed: %v", failed.Err)
		}
		t.Fatalf("inline project move result = %T, want TaskLineEditedMsg", raw)
	}
	if edited.Result.Task.ProjectID != destination.ID {
		t.Fatalf("inline project move destination = %q, want %q", edited.Result.Task.ProjectID, destination.ID)
	}

	fresh, err := config.NewManager(filepath.Join(env.dataDir, ".tt.json")).DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("reload planning: %v", err)
	}
	wantMoved := config.ProjectTaskLocator{TaskID: moving.ID, ProjectID: destination.ID}
	wantKept := config.ProjectTaskLocator{TaskID: kept.ID, ProjectID: destination.ID}
	seenMoved, seenKept := false, false
	for _, item := range fresh.GetToday() {
		seenMoved = seenMoved || item == wantMoved
		seenKept = seenKept || item == wantKept
	}
	if len(fresh.GetToday()) != 2 || !seenMoved || !seenKept {
		t.Fatalf("Today after inline project move = %+v, want moved %+v and preserved %+v", fresh.GetToday(), wantMoved, wantKept)
	}
}
