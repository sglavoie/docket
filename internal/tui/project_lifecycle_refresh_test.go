package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type session4LifecycleFixture struct {
	app     *AppModel
	cross   *OutlinerModel
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

func attachSession4CrossOutliner(t *testing.T, app *AppModel) *OutlinerModel {
	t.Helper()
	cross := NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.selectOutlinerMode(ModeAll)
	raw := cross.loadCrossProjectsCmd()()
	loaded, ok := raw.(crossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("cross-project load returned %T", raw)
	}
	if cmd := cross.handleCrossProjectsLoaded(loaded); cmd != nil {
		runSession2AppCommand(t, app, cmd)
	}
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)
	setTestPresentationPath(app, ViewUnknown, ViewCrossProjectOutliner)
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

func session4LoadedProject(t *testing.T, cross *OutlinerModel, projectID string) *models.Project {
	t.Helper()
	for _, project := range cross.loadedProjects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	t.Fatalf("loaded project %q not found", projectID)
	return nil
}

func session4HasLoadedProject(cross *OutlinerModel, projectID string) bool {
	for _, project := range cross.loadedProjects {
		if project != nil && project.ID == projectID {
			return true
		}
	}
	return false
}

func session4HasTaskRow(cross *OutlinerModel, projectID, taskID string) bool {
	for _, row := range cross.rows {
		if row.isTaskRow() && row.ProjectID == projectID && row.task != nil && row.task.ID == taskID {
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
				fixture.app.showView(ViewProjectCreator, NewProjectCreatorModel(
					fixture.app.runtime,
					fixture.app.configManager,
					fixture.app.appService,
				))

				handled, cmd := fixture.app.handleProjectMessages(ProjectCreatedMsg{Project: created})
				if !handled || cmd == nil {
					t.Fatalf("ProjectCreatedMsg handled=%v cmd=%v, want returned refresh chain", handled, cmd)
				}
				runSession2AppCommand(t, fixture.app, cmd)

				if fixture.app.currentView() != ViewCrossProjectOutliner {
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
			name: "create with task move",
			run: func(t *testing.T) {
				source := newSession4Project(t, "Inbox")
				fixture := newSession4LifecycleFixture(t, source)
				liveSource := fixture.app.projectByID(source.ID)
				moved := liveSource.Tasks[0]
				created := newSession4EmptyProject(t, fixture.dataDir, "Moved")
				session4CommitCatalogProject(t, fixture.app, created)
				fixture.app.pendingProjectMoveTask = moved
				fixture.app.pendingProjectMoveSource = liveSource
				fixture.app.showView(ViewProjectCreator, NewProjectCreatorModel(
					fixture.app.runtime,
					fixture.app.configManager,
					fixture.app.appService,
				))

				_, cmd := fixture.app.handleProjectMessages(ProjectCreatedMsg{Project: created})
				runSession2AppCommand(t, fixture.app, cmd)

				loadedDest := session4LoadedProject(t, fixture.cross, created.ID)
				if loadedDest.GetTaskByID(moved.ID) == nil || !session4HasTaskRow(fixture.cross, created.ID, moved.ID) {
					t.Fatal("create-with-move result was not visible in the destination rows")
				}
				loadedSource := session4LoadedProject(t, fixture.cross, source.ID)
				if loadedSource.GetTaskByID(moved.ID) != nil {
					t.Fatal("create-with-move left the task visible in the source project")
				}
				freshDest := &models.Project{ID: created.ID, Name: created.Name, Slug: created.Slug, Path: created.Path}
				if err := storage.LoadProject(freshDest); err != nil {
					t.Fatalf("load created destination: %v", err)
				}
				if freshDest.GetTaskByID(moved.ID) == nil {
					t.Fatal("create-with-move did not persist the destination task")
				}
			},
		},
		{
			name: "rename and context edit",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Original")
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)
				fixture.cross.pushProjectScope(project.ID)
				fixture.cross.cursor = session2TaskRow(t, fixture.cross, project.ID, project.Tasks[0].ID)
				modeBefore := fixture.cross.activeOutlinerMode()
				live.Name = "Renamed Project"
				live.Context = "office"
				session4CommitCatalogProject(t, fixture.app, live)
				fixture.app.showView(ViewProjectEditor, NewProjectEditorModel(
					live,
					fixture.app.configManager,
					fixture.app.runtime,
					fixture.app.appService,
					false,
				))

				_, cmd := fixture.app.handleProjectMessages(ProjectUpdatedMsg{})
				runSession2AppCommand(t, fixture.app, cmd)

				loaded := session4LoadedProject(t, fixture.cross, project.ID)
				if loaded.Name != "Renamed Project" || loaded.Context != "office" {
					t.Fatalf("visible metadata = name %q context %q", loaded.Name, loaded.Context)
				}
				if !session4HasTaskRow(fixture.cross, project.ID, project.Tasks[0].ID) {
					t.Fatal("renamed project task row disappeared")
				}
				if fixture.cross.activeOutlinerMode() != modeBefore {
					t.Fatal("metadata reload changed the active mode")
				}
				if fixture.cross.scopedProjectID != project.ID {
					t.Fatalf("metadata reload changed valid scope to %q", fixture.cross.scopedProjectID)
				}
				cursorRow := fixture.cross.rows[fixture.cross.cursor]
				if !cursorRow.isTaskRow() || cursorRow.task == nil ||
					cursorRow.ProjectID != project.ID || cursorRow.task.ID != project.Tasks[0].ID {
					t.Fatalf("metadata reload changed cursor identity: %#v", cursorRow)
				}
				disk := session4DiskProject(t, fixture.dataDir, project.ID)
				if disk.Name != loaded.Name || disk.Context != loaded.Context {
					t.Fatalf("durable metadata = name %q context %q", disk.Name, disk.Context)
				}
			},
		},
		{
			name: "archive and scope reconcile",
			run: func(t *testing.T) {
				project := newSession4Project(t, "Archive Target")
				fixture := newSession4LifecycleFixture(t, project)
				live := fixture.app.projectByID(project.ID)
				fixture.cross.pushProjectScope(live.ID)

				_, cmd := fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: live})
				runSession2AppCommand(t, fixture.app, cmd)

				if fixture.app.currentView() != ViewCrossProjectOutliner {
					t.Fatalf("archive popped the clean Outliner root to %s", fixture.app.currentView())
				}
				if fixture.cross.scopedProjectID != "" {
					t.Fatalf("archived scope was not reconciled: %q", fixture.cross.scopedProjectID)
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

				_, cmd := fixture.app.handleProjectMessages(ProjectRestoredMsg{Project: live})
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
				fixture.app.views.Register(ViewHelp, NewHelpViewModel(nil, ViewCrossProjectOutliner, "Help"))
				setTestPresentationPath(fixture.app, ViewUnknown, ViewCrossProjectOutliner, ViewHelp)

				_, cmd := fixture.app.handleProjectMessages(ProjectRestoredMsg{Project: live})
				if fixture.cross.crossLoading {
					t.Fatal("covered mutation started an unroutable cross-project reload")
				}
				if session4HasLoadedProject(fixture.cross, project.ID) {
					t.Fatal("covered mutation changed visible rows before reveal")
				}
				if cmd == nil {
					t.Fatal("covered unarchive should still return its status command")
				}

				_, revealCmd := fixture.app.Update(BackMsg{})
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
