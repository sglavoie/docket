package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/status"
)

type session4LifecycleFixture struct {
	app     *AppModel
	cross   *outliner.OutlinerModel
	manager *config.Manager
	dataDir string
}

func newSession4LifecycleFixture(
	t *testing.T,
	projects ...*models.Project,
) *session4LifecycleFixture {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	dataDir := t.TempDir()
	for _, project := range projects {
		if project == nil {
			continue
		}
		project.Path = filepath.Join(dataDir, "projects", project.Slug)
		if err := os.MkdirAll(project.Path, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", project.Path, err)
		}
		for _, task := range project.Tasks {
			if task != nil {
				task.Project = project
			}
		}
		project.BuildTaskTree()
		if err := storage.SaveProject(project); err != nil {
			t.Fatalf("SaveProject(%q): %v", project.ID, err)
		}
	}

	seed := newTestConfigAt(dataDir)
	setTestProjects(seed, projects)
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if err := saveTestRuntime(manager, seed); err != nil {
		t.Fatalf("save lifecycle runtime: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	ledger := workspaceLedgerFromConfig(runtime)
	service := newTUIAppService(dataDir, manager.RuntimeDependencies(), ledger)

	app := newTestAppModel()
	app.configManager = manager
	app.runtime = runtime
	app.deps = newShellDeps(ledger)
	app.appService = service
	app.projects = make(map[string]*models.Project, len(projects))
	app.syncProjectCache()
	for _, project := range app.projects {
		if err := app.persister.Load(project); err != nil {
			t.Fatalf("load lifecycle project %q: %v", project.ID, err)
		}
	}
	if len(projects) > 0 && projects[0] != nil {
		app.currentProject = app.projectByID(projects[0].ID)
	}

	cross := attachSession4CrossOutliner(t, app)
	return &session4LifecycleFixture{
		app:     app,
		cross:   cross,
		manager: manager,
		dataDir: dataDir,
	}
}

func attachSession4CrossOutliner(t *testing.T, app *AppModel) *outliner.OutlinerModel {
	t.Helper()
	cross := outliner.NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.SelectOutlinerMode(chrome.ModeAll)
	raw := cross.LoadCrossProjectsCmd()()
	loaded, ok := raw.(outliner.CrossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("cross-project load returned %T", raw)
	}
	if cmd := cross.HandleCrossProjectsLoaded(loaded); cmd != nil {
		runSession2AppCommand(t, app, cmd)
	}
	app.crossOutliner = cross
	app.views.Register(presentation.ViewCrossProjectOutliner, cross)
	setTestPresentationPath(app, presentation.ViewUnknown, presentation.ViewCrossProjectOutliner)
	return cross
}

func newSession4Project(t *testing.T, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(name+"-project", name, "")
	if err != nil {
		t.Fatalf("NewProject(%q): %v", name, err)
	}
	task, err := models.NewGenericTask(name+"-task", 1, name+" task")
	if err != nil {
		t.Fatalf("NewTask(%q): %v", name, err)
	}
	task.WorkflowState = models.StateBacklog
	task.Project = project
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()
	return project
}

func newSession4EmptyProject(t *testing.T, dataDir, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(name+"-project", name, filepath.Join(dataDir, "projects", models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject(%q): %v", name, err)
	}
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
	return project
}

func session4LoadedProject(t *testing.T, cross *outliner.OutlinerModel, projectID string) *models.Project {
	t.Helper()
	for _, project := range cross.Population.Projects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	t.Fatalf("loaded project %q not found", projectID)
	return nil
}

func session4HasLoadedProject(cross *outliner.OutlinerModel, projectID string) bool {
	for _, project := range cross.Population.Projects {
		if project != nil && project.ID == projectID {
			return true
		}
	}
	return false
}

func session4HasTaskRow(cross *outliner.OutlinerModel, projectID, taskID string) bool {
	for _, row := range cross.Rows {
		if row.IsTaskRow() && row.ProjectID == projectID && row.Task != nil && row.Task.ID == taskID {
			return true
		}
	}
	return false
}

func session4CommitCatalogProject(t *testing.T, app *AppModel, project *models.Project) {
	t.Helper()
	if err := commitCatalogToRuntime(app.runtime, func(catalog *config.WorkspaceCatalog) error {
		return catalog.UpsertProject(project)
	}); err != nil {
		t.Fatalf("commit project catalog metadata: %v", err)
	}
}

func session4DiskProject(t *testing.T, dataDir, projectID string) *models.Project {
	t.Helper()
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload lifecycle runtime: %v", err)
	}
	return testProjectByID(t, runtime.AllProjects(), projectID)
}

//nolint:gocyclo // A single table-driven lifecycle matrix keeps reload invariants comparable.
func TestProjectLifecycleRefreshMatrix(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "create",
			run: func(t *testing.T) {
				fixture := newSession4LifecycleFixture(t)
				created := newSession4EmptyProject(t, fixture.dataDir, "Created")
				session4CommitCatalogProject(t, fixture.app, created)
				fixture.app.showView(presentation.ViewProjectCreator, projecthub.NewProjectCreatorModel(
					fixture.app.runtime,
					fixture.app.configManager,
					fixture.app.appService,
				))

				handled, cmd := fixture.app.handleProjectMessages(projecthub.ProjectCreatedMsg{Project: created})
				if !handled || cmd == nil {
					t.Fatalf("ProjectCreatedMsg handled=%v cmd=%v, want returned refresh chain", handled, cmd)
				}
				runSession2AppCommand(t, fixture.app, cmd)

				if fixture.app.currentView() != presentation.ViewCrossProjectOutliner {
					t.Fatalf("create returned to %s, want cross-project Outliner", fixture.app.currentView())
				}
				session4LoadedProject(t, fixture.cross, created.ID)
				if fixture.app.projectByID(created.ID) == nil {
					t.Fatal("created project missing from the live project cache")
				}
				if disk := session4DiskProject(t, fixture.dataDir, created.ID); disk.Name != created.Name {
					t.Fatalf("durable created project = %#v", disk)
				}
			},
		},
		{
			name: "rename and description edit",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Original")
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)
				fixture.cross.PushProjectScope(project.ID)
				fixture.cross.Cursor = session2TaskRow(t, fixture.cross, project.ID, project.Tasks[0].ID)
				modeBefore := fixture.cross.ActiveOutlinerMode()
				live.Name = "Renamed Project"
				live.Description = "office"
				session4CommitCatalogProject(t, fixture.app, live)
				fixture.app.showView(presentation.ViewProjectEditor, projecthub.NewProjectEditorModel(
					live,
					fixture.app.configManager,
					fixture.app.runtime,
					fixture.app.appService,
					false,
				))

				_, cmd := fixture.app.handleProjectMessages(projecthub.ProjectUpdatedMsg{})
				runSession2AppCommand(t, fixture.app, cmd)

				loaded := session4LoadedProject(t, fixture.cross, project.ID)
				if loaded.Name != "Renamed Project" || loaded.Description != "office" {
					t.Fatalf("visible metadata = name %q description %q", loaded.Name, loaded.Description)
				}
				if !session4HasTaskRow(fixture.cross, project.ID, project.Tasks[0].ID) {
					t.Fatal("renamed project task row disappeared")
				}
				if fixture.cross.ActiveOutlinerMode() != modeBefore {
					t.Fatal("metadata reload changed the active mode")
				}
				if fixture.cross.Scope.ProjectID != project.ID {
					t.Fatalf("metadata reload changed valid scope to %q", fixture.cross.Scope.ProjectID)
				}
				cursorRow := fixture.cross.Rows[fixture.cross.Cursor]
				if !cursorRow.IsTaskRow() || cursorRow.Task == nil ||
					cursorRow.ProjectID != project.ID || cursorRow.Task.ID != project.Tasks[0].ID {
					t.Fatalf("metadata reload changed cursor identity: %#v", cursorRow)
				}
				disk := session4DiskProject(t, fixture.dataDir, project.ID)
				if disk.Name != loaded.Name || disk.Description != loaded.Description {
					t.Fatalf("durable metadata = name %q description %q", disk.Name, disk.Description)
				}
			},
		},
		{
			name: "archive and scope reconcile",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Archive Target")
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)
				fixture.cross.PushProjectScope(live.ID)

				_, cmd := fixture.app.handleProjectMessages(projecthub.ProjectArchivedMsg{Project: live})
				runSession2AppCommand(t, fixture.app, cmd)

				if fixture.app.currentView() != presentation.ViewCrossProjectOutliner {
					t.Fatalf("archive popped the clean Outliner root to %s", fixture.app.currentView())
				}
				if fixture.cross.Scope.ProjectID != "" {
					t.Fatalf("archived scope was not reconciled: %q", fixture.cross.Scope.ProjectID)
				}
				if session4HasLoadedProject(fixture.cross, project.ID) {
					t.Fatal("archived project remained in the visible active-project cache")
				}
				if disk := session4DiskProject(t, fixture.dataDir, project.ID); !disk.Archived {
					t.Fatal("archive was not durable")
				}
			},
		},
		{
			name: "unarchive",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Unarchive")
				project.Archive()
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)

				_, cmd := fixture.app.handleProjectMessages(projecthub.ProjectRestoredMsg{Project: live})
				runSession2AppCommand(t, fixture.app, cmd)

				session4LoadedProject(t, fixture.cross, project.ID)
				if disk := session4DiskProject(t, fixture.dataDir, project.ID); disk.Archived {
					t.Fatal("unarchive was not durable")
				}
			},
		},
		{
			name: "mutation while another destination is active",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Covered Restore")
				project.Archive()
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)
				fixture.app.views.Register(presentation.ViewHelp, help.NewHelpViewModel(nil, "Help"))
				setTestPresentationPath(fixture.app, presentation.ViewUnknown, presentation.ViewCrossProjectOutliner, presentation.ViewHelp)

				_, cmd := fixture.app.handleProjectMessages(projecthub.ProjectRestoredMsg{Project: live})
				if fixture.cross.Population.Loading {
					t.Fatal("covered mutation started an unroutable cross-project reload")
				}
				if session4HasLoadedProject(fixture.cross, project.ID) {
					t.Fatal("covered mutation changed visible rows before reveal")
				}
				if cmd == nil {
					t.Fatal("covered unarchive should still return its status command")
				}

				_, revealCmd := fixture.app.Update(status.BackMsg{})
				runSession2AppCommand(t, fixture.app, revealCmd)

				session4LoadedProject(t, fixture.cross, project.ID)
				if disk := session4DiskProject(t, fixture.dataDir, project.ID); disk.Archived {
					t.Fatal("covered unarchive was not durable")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}
