package tui

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestTUISetDueDateTodayMovesTaskToTodayAtomically(t *testing.T) {
	m, repo, task, dataDir := newPlanningLedgerApp(t, models.StateWaiting)
	m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)
	due := time.Now()

	cmd := m.secondaryFeatures.Bulk.SetDueDateLocators([]app.TaskLocator{{
		ProjectID: m.currentProject.ID,
		TaskID:    task.ID,
	}}, due)
	if cmd == nil {
		t.Fatal("SetDueDateLocators returned nil")
	}
	raw := cmd()
	msg, ok := raw.(coordinator.BatchTransactionResultMsg)
	if !ok {
		t.Fatalf("due-date result = %T, want BatchTransactionResultMsg", raw)
	}

	if got := diskTodayLen(t, dataDir); got != 1 {
		t.Fatalf("Today entries = %d, want 1", got)
	}
	if got := diskNextOrderLen(t, dataDir); got != 0 {
		t.Fatalf("Next entries = %d, want 0", got)
	}
	if got := diskTaskState(t, m.currentProject, task.ID); got != models.StateBacklog {
		t.Fatalf("state = %s, want backlog", got)
	}
	artifactCount := 0
	if len(repo.lastAttempt) == 1 {
		artifactCount = len(repo.lastAttempt[0].Artifacts)
	}
	if len(repo.lastAttempt) != 1 || artifactCount != 1 {
		t.Fatalf("changesets/artifacts = %d/%d, want one atomic task+planning commit", len(repo.lastAttempt), artifactCount)
	}

	m.handleBatchTransactionResult(msg)
	identity := taskref.TaskIdentity{ProjectID: m.currentProject.ID, TaskID: task.ID}
	if !m.todayIDs[identity] {
		t.Fatalf("runtime Today index = %+v, want %v", m.todayIDs, identity)
	}
}

// failableScenarioRepo wraps the storage-backed scenarioRepo so a test can make
// the next Commit fail atomically (no partial write) and inspect the exact
// changesets a mutation attempted, proving the task write and the planning
// artifact rode ONE transaction.
type failableScenarioRepo struct {
	*scenarioRepo
	fail        bool
	lastAttempt []storage.ChangeSet
}

func (r *failableScenarioRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	r.lastAttempt = append([]storage.ChangeSet(nil), changesets...)
	if r.fail {
		return nil, fmt.Errorf("injected commit failure")
	}
	return r.scenarioRepo.Commit(changesets...)
}

// newPlanningLedgerApp builds a ledger-backed AppModel composed with the SAME
// default planning-artifact builder NewAppModel installs, plus the secondary
// coordinators, so section and Today/Next moves fold their planning
// reconciliation into the durable workspace-ledger transaction exactly as
// production does. The single task starts in state, hosted under the Outliner via
// a QueueToggleable so the q/w handlers can resolve it. It returns the model, the
// wrapping repo (for failure injection), the task, and the data dir for on-disk
// assertions.
func newPlanningLedgerApp(t *testing.T, state models.WorkflowState) (*AppModel, *failableScenarioRepo, *models.Task, string) {
	t.Helper()

	dataDir := t.TempDir()
	task := seedTask(t, 1, "Move target", state)

	project, err := models.NewProject("planning-txn-proj", "planning-txn", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "planning-txn"
	project.Path = filepath.Join(dataDir, "projects", "planning-txn")
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := &failableScenarioRepo{scenarioRepo: newScenarioRepo(commitSvc, project)}
	repo.seed(t)

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil).
		WithPlanningArtifacts(coordinator.ManagerPlanningArtifacts(manager))

	appModel := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		deps:              newShellDeps(ledger),
		projects:          map[string]*models.Project{project.ID: project},
		persister:         coordinator.NewProjectService(),
		appService:        svc,
		secondaryFeatures: coordinator.NewSecondaryCoordinators(svc),
		width:             80,
		height:            24,
		views:             route.NewViewRegistry(),
		currentProject:    project,
		nextOrder:         []outliner.PlanningTask{},
		nextOrderIDs:      make(map[taskref.TaskIdentity]bool),
		today:             []outliner.PlanningTask{},
		todayIDs:          make(map[taskref.TaskIdentity]bool),
	}
	appModel.showView(presentation.ViewCrossProjectOutliner, &invariantQueueView{taskID: task.ID, project: project})
	return appModel, repo, task, dataDir
}

func (m *AppModel) seedDiskNextOrder(t *testing.T, taskID, projectID string) {
	t.Helper()
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetNext([]config.ProjectTaskLocator{{TaskID: taskID, ProjectID: projectID}})
	}); err != nil {
		t.Fatalf("seed focus queue: %v", err)
	}
}

func (m *AppModel) seedDiskToday(t *testing.T, taskID, projectID string) {
	t.Helper()
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetToday([]config.ProjectTaskLocator{{TaskID: taskID, ProjectID: projectID}})
	}); err != nil {
		t.Fatalf("seed daily plan: %v", err)
	}
}

// TestPlanningNextToTodayIsAtomicAndDurable locks FR-010-001: the TUI w move from
// Next to Today commits the workflow-state change, the Next order removal, and the
// Today insertion as ONE workspace-ledger transaction, so the move is durable
// (survives a restart), reversible in exact order, and atomic under an injected
// failure.
func TestPlanningNextToTodayIsAtomicAndDurable(t *testing.T) {
	t.Run("w move folds state, Next removal, and Today insertion into one durable transaction", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateWaiting)
		// Precondition: the task holds a Next order entry, so the move must prove the
		// order drop rides the same transaction as the state change and Today add.
		m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("precondition Next order entries = %d, want 1", got)
		}

		if cmd := m.toggleToday(nil); cmd == nil {
			t.Fatal("toggleToday returned nil")
		}

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("state after move = %s, want backlog", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after move = %d, want 0 (dropped in same txn)", got)
		}
		if got := diskTodayLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after move = %d, want 1 (Today added in same txn)", got)
		}

		// Durability: a fresh ledger over the same data dir (a restart) undoes the
		// whole move, restoring the exact workflow state, Next ordering, and Today.
		fresh := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
		if _, _, err := fresh.UndoWithEntry(storage.GlobalScope()); err != nil {
			t.Fatalf("durable undo from fresh ledger: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after durable undo = %s, want waiting", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("Next order after durable undo = %d, want 1 (restored)", got)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after durable undo = %d, want 0 (Today removed)", got)
		}

		if _, _, err := fresh.RedoWithEntry(storage.GlobalScope()); err != nil {
			t.Fatalf("durable redo from fresh ledger: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("state after durable redo = %s, want backlog", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after durable redo = %d, want 0", got)
		}
		if got := diskTodayLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after durable redo = %d, want 1", got)
		}
	})

	t.Run("in-session undo and redo restore the exact horizons without an artifact conflict", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateWaiting)
		m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)

		_ = m.toggleToday(nil)
		assertExclusiveMembership(t, m, task.ID, false, true)

		if cmd := m.handleUndoToggle(); cmd == nil {
			t.Fatal("handleUndoToggle returned nil (undo did not fire)")
		}
		assertExclusiveMembership(t, m, task.ID, true, false)
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after undo = %d, want 0", got)
		}
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("Next order after undo = %d, want 1", got)
		}

		if cmd := m.handleRedoToggle(); cmd == nil {
			t.Fatal("handleRedoToggle returned nil (redo did not fire)")
		}
		assertExclusiveMembership(t, m, task.ID, false, true)
		if got := diskTodayLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after redo = %d, want 1", got)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after redo = %d, want 0", got)
		}
	})

	t.Run("an injected commit failure leaves project and planning at pre-mutation values", func(t *testing.T) {
		m, repo, task, dataDir := newPlanningLedgerApp(t, models.StateWaiting)
		m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)

		repo.fail = true
		if cmd := m.toggleToday(nil); cmd == nil {
			t.Fatal("expected an error command from the failed move")
		}

		// The attempted transaction folded the tasks.md write and the planning.json
		// artifact into ONE changeset, so a failure cannot land a partial move.
		if len(repo.lastAttempt) != 1 {
			t.Fatalf("attempted changesets = %d, want 1 (atomic fold)", len(repo.lastAttempt))
		}
		cs := repo.lastAttempt[0]
		if cs.Project == nil {
			t.Fatal("attempted changeset carried no tasks.md write")
		}
		if !changeSetHasPlanningArtifact(cs) {
			t.Fatalf("attempted changeset carried no planning.json artifact, paths=%v", artifactPaths(cs))
		}

		// Disk is untouched: state, Next ordering, and Today all at pre-mutation.
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after failed move = %s, want waiting (unchanged)", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("Next order after failed move = %d, want 1 (unchanged)", got)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after failed move = %d, want 0 (unchanged)", got)
		}
	})
}

// TestStateMovesPreservePlanningExclusivity locks the schema-6 TUI seams:
// explicit Next planning drops Today, and lifecycle moves clear stale planning
// membership in the same durable transaction.
func TestStateMovesPreservePlanningExclusivity(t *testing.T) {
	t.Run("explicit AddNext drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.ApplyPlanning(app.PlanningRequest{
			Locator: app.TaskLocator{ProjectID: m.currentProject.ID, TaskID: task.ID},
			Action:  app.PlanningAddNext,
		}); err != nil {
			t.Fatalf("ApplyPlanning(AddNext): %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateBacklog {
			t.Fatalf("state = %s, want backlog", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Today after AddNext = %d, want 0", got)
		}
		if got := diskNextOrderLen(t, dataDir); got != 1 {
			t.Fatalf("Next after AddNext = %d, want 1", got)
		}
	})

	t.Run("generic transition out of Next drops the stale order entry", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.Transition(app.StateRequest{Locator: app.TaskLocator{ProjectID: m.currentProject.ID, TaskID: task.ID}, Target: models.StateWaiting}); err != nil {
			t.Fatalf("Transition: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after leaving Next = %d, want 0", got)
		}
	})

	t.Run("coordinated lifecycle transition drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		cmd := m.secondaryFeatures.TaskItem.Transition(
			taskref.For(task, m.currentProject),
			models.StateWaiting,
		)
		assertNotTransactionError(t, cmd())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after section move = %d, want 0", got)
		}
	})

	t.Run("project commit lifecycle move drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		// Mirror the shell bulk-state handler: mutate the project in memory, then
		// commit through the project coordinator naming the changed tasks.
		m.currentProject.MutateTask(task.ID, func(tk *models.Task) {
			tk.ApplyWorkflowState(models.StateWaiting)
		})
		cmd := m.secondaryFeatures.TaskItem.CommitProject(
			m.currentProject,
			app.ProjectCommitOptions{UndoAction: "Change State", PlanningTaskIDs: []string{task.ID}},
			"moved",
		)
		assertNotTransactionError(t, cmd())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after bulk section move = %d, want 0", got)
		}
	})
}

// changeSetHasPlanningArtifact reports whether a changeset carries the planning.json
// domain artifact.
func changeSetHasPlanningArtifact(cs storage.ChangeSet) bool {
	for _, art := range cs.Artifacts {
		if filepath.Base(art.Path) == "planning.json" {
			return true
		}
	}
	return false
}

func artifactPaths(cs storage.ChangeSet) []string {
	paths := make([]string, 0, len(cs.Artifacts))
	for _, art := range cs.Artifacts {
		paths = append(paths, art.Path)
	}
	return paths
}
