package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newTrashRestoreTestApp(
	t *testing.T,
	refs config.ProjectReferences,
) (*AppModel, *models.Project, string) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	seed := config.NewRuntimeState(dataDir, config.NewDomains())
	if err := saveTestRuntime(manager, seed); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}

	projectPath := filepath.Join(dataDir, "projects", "restored-project")
	project, err := models.NewProject("restore-project-id", "Restored Project", projectPath)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	rawRefs, err := json.Marshal(refs)
	if err != nil {
		t.Fatalf("marshal refs: %v", err)
	}
	entryID, err := storage.MoveProjectToTrash(project, dataDir, "restore state test", rawRefs)
	if err != nil {
		t.Fatalf("MoveProjectToTrash: %v", err)
	}

	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	service := newTUIAppService(dataDir, manager.RuntimeDependencies())
	app := &AppModel{
		configManager:            manager,
		runtime:                  runtime,
		deps:                     newShellDeps(runtime, service, manager),
		appService:               service,
		projects:                 make(map[string]*models.Project),
		persister:                NewProjectService(),
		viewStack:                []ViewType{ViewDashboard, ViewTrashedProjects},
		views:                    NewViewRegistry(),
		width:                    80,
		height:                   24,
		dailyPlanIDs:             make(map[string]bool),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectLastView:          make(map[string]ViewType),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
	}
	return app, project, entryID
}

func TestRestoreProjectFromTrashCommandUsesImmutableCapturedState(t *testing.T) {
	app, project, entryID := newTrashRestoreTestApp(t, config.ProjectReferences{WasCurrent: true})
	cmd := app.restoreProjectFromTrash(entryID)
	if cmd == nil {
		t.Fatal("restore command is nil")
	}

	resultCh := make(chan tea.Msg, 1)
	go func() { resultCh <- cmd() }()

	// Exercise the live model while the command owns the filesystem transaction.
	// Under -race this catches any command-side RuntimeState, project-cache, or
	// current-project access. The command must use only the snapshot captured
	// before it was launched.
	for {
		select {
		case result := <-resultCh:
			msg, ok := result.(projectRestoredFromTrashMsg)
			if !ok {
				t.Fatalf("restore command returned %T, want projectRestoredFromTrashMsg", result)
			}
			if msg.Project == nil || msg.Project.ID != project.ID {
				t.Fatalf("restored project = %+v, want %q", msg.Project, project.ID)
			}
			if !msg.References.WasCurrent {
				t.Fatal("restore result lost captured machine-reference data")
			}
			if app.runtime.GetProject(project.ID) != nil {
				t.Fatal("async command mutated the live runtime snapshot")
			}
			return
		default:
			app.projects["volatile"] = project
			delete(app.projects, "volatile")
			app.currentProject = project
			app.currentProject = nil
			app.runtime.SetMachineInMemory(config.NewMachineState())
			app.runtime.SetCatalogInMemory(config.NewWorkspaceCatalog())
		}
	}
}

func TestRestoreProjectFromTrashMachineFailureRefreshesAndRetries(t *testing.T) {
	viewState := config.ProjectViewState{FilterParked: true}
	refs := config.ProjectReferences{
		WasCurrent:       true,
		ViewState:        &viewState,
		RecentTaskVisits: []config.RecentTaskVisit{{TaskID: "restored-task"}},
		InCycleProjects:  true,
		ContextCycleKeys: []string{"work"},
		FocusQueue:       []config.FocusQueueItem{{TaskID: "restored-task"}},
	}
	app, project, entryID := newTrashRestoreTestApp(t, refs)
	result := app.restoreProjectFromTrash(entryID)()
	msg, ok := result.(projectRestoredFromTrashMsg)
	if !ok {
		t.Fatalf("restore command returned %T, want projectRestoredFromTrashMsg", result)
	}

	// A directory at the adjacent lock path lets refresh read machine.json but
	// makes CommitMachine fail deterministically, including when tests run as a
	// privileged user. Removing it makes the exact same recovery payload retryable.
	machineLock := config.MachineStatePath(app.runtime.DataDir()) + ".lock"
	if err := os.Remove(machineLock); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove machine lock: %v", err)
	}
	if err := os.Mkdir(machineLock, 0o700); err != nil {
		t.Fatalf("replace machine lock with directory: %v", err)
	}

	handled, retryCmd := app.handleProjectMessages(msg)
	if !handled {
		t.Fatal("projectRestoredFromTrashMsg was not handled")
	}
	if retryCmd == nil {
		t.Fatal("partial machine recovery did not schedule a retry")
	}
	if app.err == nil || !strings.Contains(app.err.Error(), "machine-state recovery is incomplete") {
		t.Fatalf("partial restore error = %v, want incomplete machine-state recovery", app.err)
	}
	if app.statusMessage == "Restored: Restored Project" {
		t.Fatal("partial restore reported a success toast")
	}
	if _, ok := app.pendingTrashRestoreRecovery[project.ID]; !ok {
		t.Fatal("machine-reference recovery payload was discarded after commit failure")
	}
	if app.runtime.GetProject(project.ID) == nil {
		t.Fatal("durable runtime was not refreshed after the project restore")
	}
	if app.projects[project.ID] != msg.Project {
		t.Fatal("project cache did not retain the fully loaded restore result")
	}
	if got := app.runtime.Planning().FocusQueue; len(got) != 1 || got[0].ProjectID != project.ID {
		t.Fatalf("durable focus references = %+v, want restored project", got)
	}
	if app.runtime.Machine().CurrentProject == project.ID {
		t.Fatal("failed machine commit leaked the pending current-project reference")
	}
	if _, err := os.Stat(project.Path); err != nil {
		t.Fatalf("restored project data missing after partial machine failure: %v", err)
	}
	if _, err := storage.ReadTrashTombstone(app.runtime.DataDir(), entryID); !os.IsNotExist(err) {
		t.Fatalf("consumed tombstone still present or returned unexpected error: %v", err)
	}

	if err := os.RemoveAll(machineLock); err != nil {
		t.Fatalf("repair machine lock: %v", err)
	}
	handled, _ = app.handleProjectMessages(retryTrashRestoreMachineRefsMsg{ProjectID: project.ID})
	if !handled {
		t.Fatal("machine-reference retry message was not handled")
	}
	if app.err != nil {
		t.Fatalf("machine-reference retry failed: %v", app.err)
	}
	if _, ok := app.pendingTrashRestoreRecovery[project.ID]; ok {
		t.Fatal("successful machine-reference retry retained stale recovery data")
	}
	machine := app.runtime.Machine()
	if machine.CurrentProject != project.ID {
		t.Fatalf("CurrentProject = %q, want %q", machine.CurrentProject, project.ID)
	}
	if state, ok := machine.ProjectViewStates[project.ID]; !ok || !state.FilterParked {
		t.Fatalf("restored view state = %+v (ok=%v), want parked", state, ok)
	}
	if len(machine.RecentTaskVisits) != 1 || machine.RecentTaskVisits[0].ProjectID != project.ID {
		t.Fatalf("restored recent visits = %+v", machine.RecentTaskVisits)
	}
	if len(machine.CycleProjects) != 1 || machine.CycleProjects[0] != project.ID {
		t.Fatalf("restored cycle projects = %+v", machine.CycleProjects)
	}
	if app.currentProject != app.projects[project.ID] {
		t.Fatal("main update handler did not restore the current-project cache pointer")
	}
	if app.statusMessage != "Restored: Restored Project" {
		t.Fatalf("success status = %q, want final restored toast", app.statusMessage)
	}
}
