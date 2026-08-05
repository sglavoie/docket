package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestTUIWorkspaceLedgerUndoRedoForApplicationMutations is the FR-011-001
// regression: general u/ctrl+r must restore the newest application-service
// workspace-ledger transaction, not a stale legacy per-project entry, and the
// restore must survive a restart. Each case commits an application mutation
// through the ledger-backed service, discards the mutating AppModel entirely
// (simulating a restart with no in-memory undo state), then drives u/ctrl+r
// through the real key path on a fresh AppModel and asserts one undo and one redo
// restore the exact on-disk state.
func TestTUIWorkspaceLedgerUndoRedoForApplicationMutations(t *testing.T) {
	newTitle := "Edited via application service"

	attributeCases := []struct {
		name   string
		mutate func(svc *app.Service, taskID string) error
	}{
		{
			name: "edit",
			mutate: func(svc *app.Service, taskID string) error {
				_, err := svc.Edit(app.EditRequest{TaskID: taskID, Title: &newTitle})
				return err
			},
		},
		{
			name: "priority",
			mutate: func(svc *app.Service, taskID string) error {
				_, err := svc.SetPriority(taskID, 2)
				return err
			},
		},
		{
			name: "completion",
			mutate: func(svc *app.Service, taskID string) error {
				_, err := svc.Complete(taskID)
				return err
			},
		},
	}

	for _, tc := range attributeCases {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			task := seedTask(t, 1, "Ledger undo target", models.StateBacklog)
			project := newLedgerUndoProject(t, dataDir, "ledger-undo", task)

			mutator := newLedgerUndoApp(t, dataDir, project, task.ID, true)
			before := readTasksFile(t, project)

			if err := tc.mutate(mutator.appService, task.ID); err != nil {
				t.Fatalf("application mutation: %v", err)
			}
			after := readTasksFile(t, project)
			if bytes.Equal(before, after) {
				t.Fatal("application mutation did not change tasks.md on disk")
			}

			// Simulate a restart: a brand-new AppModel with no in-memory undo state,
			// reading the durable ledger the previous session wrote.
			restarted := newLedgerUndoApp(t, dataDir, restartLedgerProject(project), task.ID, false)

			restarted = sendAppKey(restarted, syntheticKey('u'))
			if got := readTasksFile(t, project); !bytes.Equal(got, before) {
				t.Fatalf("undo did not restore exact disk state\n before: %q\n after undo: %q", before, got)
			}

			restarted = sendAppKey(restarted, syntheticCtrlKey('r'))
			if got := readTasksFile(t, project); !bytes.Equal(got, after) {
				t.Fatalf("redo did not restore exact disk state\n after mutation: %q\n after redo: %q", after, got)
			}
			_ = restarted
		})
	}

	t.Run("planning-aware transition", func(t *testing.T) {
		dataDir := t.TempDir()
		task := seedTask(t, 1, "Planning transition target", models.StateBacklog)
		project := newLedgerUndoProject(t, dataDir, "planning-undo", task)

		mutator := newLedgerUndoApp(t, dataDir, project, task.ID, true)

		// Seed the task into Today so a lifecycle transition to Waiting must drop
		// the Today entry through the folded planning artifact.
		seedTodayMembership(t, mutator.configManager, project.ID, task.ID)
		if !planningTaskInToday(t, mutator.configManager, project.ID, task.ID) {
			t.Fatal("failed to seed Today membership")
		}
		before := readTasksFile(t, project)

		if _, err := mutator.appService.Transition(app.StateRequest{TaskID: task.ID, Target: models.StateWaiting}); err != nil {
			t.Fatalf("planning-aware transition: %v", err)
		}
		if planningTaskInToday(t, mutator.configManager, project.ID, task.ID) {
			t.Fatal("transition into Next did not drop the Today entry on disk")
		}
		after := readTasksFile(t, project)

		restarted := newLedgerUndoApp(t, dataDir, restartLedgerProject(project), task.ID, false)

		restarted = sendAppKey(restarted, syntheticKey('u'))
		if got := readTasksFile(t, project); !bytes.Equal(got, before) {
			t.Fatalf("undo did not restore exact tasks.md state\n before: %q\n after undo: %q", before, got)
		}
		if !planningTaskInToday(t, restarted.configManager, project.ID, task.ID) {
			t.Fatal("undo did not restore the Today (Daily Plan) entry on disk")
		}
		if fresh := restarted.currentProject.GetTaskByID(task.ID); fresh == nil || fresh.State() != models.StateBacklog {
			t.Fatalf("undo did not restore pre-transition workflow state: %v", fresh)
		}

		restarted = sendAppKey(restarted, syntheticCtrlKey('r'))
		if got := readTasksFile(t, project); !bytes.Equal(got, after) {
			t.Fatalf("redo did not restore exact tasks.md state\n after: %q\n after redo: %q", after, got)
		}
		if planningTaskInToday(t, restarted.configManager, project.ID, task.ID) {
			t.Fatal("redo did not re-drop the Today entry on disk")
		}
	})
}

// newLedgerUndoProject builds a single-project workspace pointer whose task tree
// is ready for the ledger-backed service to commit.
func newLedgerUndoProject(t *testing.T, dataDir, slug string, task *models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(slug+"-name", slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	project.Tasks = []*models.Task{task}
	project.NextShortID = task.ShortID + 1
	project.BuildTaskTree()
	return project
}

// restartLedgerProject returns a bare project pointer sharing only the durable
// identity of src, so newLedgerUndoApp reloads its tasks fresh from disk — the
// state a real restart would rehydrate.
func restartLedgerProject(src *models.Project) *models.Project {
	return &models.Project{ID: src.ID, Name: src.Name, Slug: src.Slug, Path: src.Path}
}

// newLedgerUndoApp constructs an AppModel over the durable workspace at dataDir
// wired to the same ledger-backed application service and planning reconciliation
// as production. With seed=true it writes the project's initial tasks.md; with
// seed=false it reloads an already-persisted project, modeling a restart.
func newLedgerUndoApp(t *testing.T, dataDir string, project *models.Project, taskID string, seed bool) *AppModel {
	t.Helper()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	if seed {
		repo.seed(t)
	} else if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload project on restart: %v", err)
	}

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	manager := config.NewManager(filepath.Join(dataDir, "tt.json"))
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	// Mirror app_init.go: fold Today/Next exclusivity into every transition so a
	// planning-aware move rides the same durable ledger transaction as the task
	// write.
	svc.WithPlanningArtifacts(planningReconcileArtifactBuilder(manager))

	appModel := &AppModel{
		configManager:     manager,
		runtime:           runtime,
		deps:              newShellDeps(ledger),
		projects:          map[string]*models.Project{project.ID: project},
		persister:         NewProjectService(),
		appService:        svc,
		width:             80,
		height:            24,
		views:             NewViewRegistry(),
		currentProject:    project,
		nextOrder:         []PlanningTask{},
		nextOrderIDs:      make(map[TaskIdentity]bool),
		today:             []PlanningTask{},
		todayIDs:          make(map[TaskIdentity]bool),
		operationRegistry: NewOperationRegistry(),
		navigation:        NewNavigationState(),
	}
	appModel.operationExecutor = NewOperationExecutor(appModel.operationRegistry)
	outliner := NewOutlinerModel(project, manager, runtime)
	outliner.rebuildRows()
	outliner.moveCursorToTask(taskID)
	appModel.showView(ViewCrossProjectOutliner, outliner)
	if err := appModel.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core operations: %v", err)
	}
	if err := appModel.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := appModel.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
	}
	if err := appModel.registerProjectOperations(); err != nil {
		t.Fatalf("register project operations: %v", err)
	}
	if err := appModel.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input strategy: %v", err)
	}
	appModel.wireOutlinerOperationExecutor(outliner)
	return appModel
}

// seedTodayMembership commits a Daily Plan (Today) entry for the task to disk.
func seedTodayMembership(t *testing.T, manager *config.Manager, projectID, taskID string) {
	t.Helper()
	planning, err := manager.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	planning.SetToday([]config.ProjectTaskLocator{{
		TaskID:    taskID,
		ProjectID: projectID,
	}})
	if err := manager.CommitPlanning(planning); err != nil {
		t.Fatalf("commit planning: %v", err)
	}
}

// planningTaskInToday reports whether the task is in the durable Daily Plan.
// Pass projectID, taskID for a qualified assertion. The one-ID compatibility
// form is retained for older single-project tests.
func planningTaskInToday(t *testing.T, manager *config.Manager, ids ...string) bool {
	t.Helper()
	planning, err := manager.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	if len(ids) == 2 {
		return todayContains(planning, TaskIdentity{ProjectID: ids[0], TaskID: ids[1]})
	}
	if len(ids) == 1 {
		for _, item := range planning.GetToday() {
			if item.TaskID == ids[0] {
				return true
			}
		}
	}
	return false
}

// readTasksFile returns the exact on-disk tasks.md bytes for the project.
func readTasksFile(t *testing.T, project *models.Project) []byte {
	t.Helper()
	data, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	return data
}
