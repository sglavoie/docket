package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newMachineLayoutTestApp builds an AppModel rooted at a real temporary data
// directory with an isolated HOME so machine-local commits actually persist to
// disk (machine.json lives under the home runtime root). The manager and runtime
// share the same workspace so a machine commit is observable through
// runtime.Machine() and a fresh reload.
func newMachineLayoutTestApp(t *testing.T) (*AppModel, *models.Project) {
	t.Helper()
	resetSessionMutations()

	dataDir := t.TempDir()
	// Keep machine.json local to the temp workspace so the test leaves no trace in
	// the real home runtime directory.
	t.Setenv("HOME", dataDir)

	project := testutil.NewTestProjectWithTasks("Layout Project")
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID

	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := saveTestRuntime(manager, cfg); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	app := &AppModel{
		configManager:        manager,
		runtime:              runtime,
		deps:                 newShellDeps(runtime, nil, manager, nil),
		persister:            NewProjectService(),
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		projectLastView:      make(map[string]ViewType),
		viewStack:            []ViewType{ViewDashboard},
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
		currentProject:       project,
		navHistory:           NewNavHistory(),
	}
	return app, project
}

// TestSaveProjectViewStateCommitsMachineDomain proves the migrated
// saveProjectViewState routes a per-project view-state change through the
// machine-local committer (no whole-config Manager.Save): the change is readable
// through the runtime snapshot and survives a fresh reload from disk.
func TestSaveProjectViewStateCommitsMachineDomain(t *testing.T) {
	app, project := newMachineLayoutTestApp(t)
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.filter.Parked = true
	app.outliner.filter.GitHubIssue = true

	app.saveProjectViewState()

	// Observable through the in-memory runtime snapshot after commit+refresh.
	vs, ok := app.runtime.Machine().GetProjectViewState(project.ID)
	if !ok || !vs.OutlinerFilterParked || !vs.OutlinerFilterGitHubIssue {
		t.Fatalf("runtime machine view state = %+v (ok=%v), want parked+github", vs, ok)
	}

	// Persisted to disk: a fresh runtime load agrees.
	reloaded, err := config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	rvs, ok := reloaded.Machine().GetProjectViewState(project.ID)
	if !ok || !rvs.OutlinerFilterParked || !rvs.OutlinerFilterGitHubIssue {
		t.Fatalf("reloaded machine view state = %+v (ok=%v), want parked+github", rvs, ok)
	}
}

// TestSaveCrossOutlinerStateCommitsMachineDomain proves the migrated
// saveCrossOutlinerState persists cross-outliner layout to the machine domain and
// that a freshly constructed outliner seeds itself back from that committed state.
func TestSaveCrossOutlinerStateCommitsMachineDomain(t *testing.T) {
	app, _ := newMachineLayoutTestApp(t)
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.filter.Parked = true

	app.saveCrossOutlinerState()

	if !app.runtime.Machine().CrossOutlinerFilterParked {
		t.Fatalf("runtime machine cross-outliner parked = false, want true")
	}

	reloaded, err := config.LoadRuntimeState(app.runtime.DataDir())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.Machine().CrossOutlinerFilterParked {
		t.Fatalf("reloaded cross-outliner parked = false, want true")
	}

	// A fresh cross outliner built from the reloaded runtime restores the filter.
	restored := NewOutlinerModel(nil, app.configManager, reloaded)
	if !restored.filter.Parked {
		t.Fatalf("restored cross-outliner should seed parked filter from machine state")
	}
}
