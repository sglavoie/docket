package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// quickSwitchServiceFixture builds a disk-backed AppModel with an application
// service over one project, plus a primed quick-switch popup pointing at the
// single task. The prefix handlers commit through the service (ledger-undoable)
// and the shell reloads the project; assertions read committed on-disk state.
func quickSwitchServiceFixture(t *testing.T) (*AppModel, *models.Project, string) {
	t.Helper()

	dataDir := t.TempDir()
	task, err := models.NewTask("qs-task", 1, models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Priority = 3

	proj := &models.Project{ID: "qs-proj", Name: "Project 1", Slug: "qs-proj", Path: filepath.Join(dataDir, "projects", "qs-proj")}
	proj.Tasks = []*models.Task{task}
	proj.NextShortID = 2
	proj.BuildTaskTree()
	if saveErr := storage.SaveProject(proj); saveErr != nil {
		t.Fatalf("save proj: %v", saveErr)
	}
	if loadErr := storage.LoadProject(proj); loadErr != nil {
		t.Fatalf("load proj: %v", loadErr)
	}

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{proj})

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, proj)
	svc := app.NewService(repo, nil, nil)

	m := &AppModel{
		runtime:    testRuntimeAt(cfg.DataDir(), cfg),
		projects:   map[string]*models.Project{proj.ID: proj},
		persister:  NewProjectService(),
		appService: svc,
	}
	m.recentTaskVisits = []RecentTaskVisitEntry{{Task: proj.Tasks[0], Project: proj}}
	initQuickSwitchFilter(m)
	m.showQuickSwitch = true
	return m, proj, task.ID
}

func TestQuickSwitchPrefixDispatch(t *testing.T) {
	t.Run("status prefix changes task status", func(t *testing.T) {
		app, proj, taskID := quickSwitchServiceFixture(t)
		app.quickSwitchPendingPrefix = ","

		msg := syntheticKey('s')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		fresh := &models.Project{ID: proj.ID, Name: proj.Name, Slug: proj.Slug, Path: proj.Path}
		if err := storage.LoadProject(fresh); err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got := fresh.GetTaskByID(taskID); got == nil || !got.IsInState(models.StateInProgress) {
			t.Errorf("expected committed state in_progress, got %+v", got)
		}
		if app.statusMessage == "" {
			t.Error("expected status message to be set")
		}
	})

	t.Run("priority prefix changes task priority", func(t *testing.T) {
		app, proj, taskID := quickSwitchServiceFixture(t)
		app.quickSwitchPendingPrefix = "!"

		msg := syntheticKey('1')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		fresh := &models.Project{ID: proj.ID, Name: proj.Name, Slug: proj.Slug, Path: proj.Path}
		if err := storage.LoadProject(fresh); err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got := fresh.GetTaskByID(taskID); got == nil || got.Priority != 1 {
			t.Errorf("expected committed priority 1, got %+v", got)
		}
	})

	t.Run("esc cancels pending prefix", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := syntheticSpecialKey(tea.KeyEscape)
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared on esc")
		}
		if !app.showQuickSwitch {
			t.Error("popup should still be open after canceling prefix")
		}
	})

	t.Run("invalid prefix key is ignored", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, WorkflowState: models.StateReady, Priority: 3}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := syntheticKey('z')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.State() != models.StateReady {
			t.Errorf("expected state unchanged (ready), got %s", task.State())
		}
	})
}
