package tui

import (
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newTestAppModel creates an AppModel for testing with a temp config file.
func newTestAppModel() *AppModel {
	resetSessionMutations()

	cfg := newTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	return &AppModel{
		configManager:        manager,
		runtime:              runtime,
		deps:                 newShellDeps(runtime, nil, manager, nil),
		projects:             make(map[string]*models.Project),
		persister:            NewProjectService(),
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		navigation:           NewNavigationState(),
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
	}
}

// newTestAppModelWithProject creates an AppModel with a test project and temp config.
func newTestAppModelWithProject() (*AppModel, *models.Project) {
	resetSessionMutations()

	cfg := newTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID

	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	app := &AppModel{
		configManager:        manager,
		runtime:              runtime,
		deps:                 newShellDeps(runtime, nil, manager, nil),
		projects:             map[string]*models.Project{project.ID: project},
		persister:            NewProjectService(),
		outlinerToggleStates: make(map[string]*OutlinerToggleState),
		navigation:           NewNavigationState(),
		width:                80,
		height:               24,
		views:                NewViewRegistry(),
		currentProject:       project,
	}

	return app, project
}

// setTestPresentationPath installs renderer identities through the same typed
// route/overlay authority as production. The fallback is only for deliberately
// partial fixtures that lack the entity data a production route requires.
func setTestPresentationPath(app *AppModel, views ...PresentationID) {
	app.navigation = NewNavigationState()
	for _, view := range views {
		if view == ViewDashboard {
			continue
		}
		if view == ViewCrossProjectOutliner {
			app.navigation.ResetRoot()
			continue
		}
		if app.installTypedPresentation(view) {
			continue
		}
		overlay, err := NewOverlay(OverlayForm, "test:"+view.String())
		if err != nil {
			panic(err)
		}
		overlay.presentation = view
		overlay.hasPresentation = true
		app.navigation.Overlays.Push(overlay, FocusTarget{}, nil)
	}
}

func pushTestPresentationOverlay(app *AppModel, view PresentationID) {
	overlay, err := NewOverlay(OverlayForm, "test:"+view.String())
	if err != nil {
		panic(err)
	}
	overlay.presentation = view
	overlay.hasPresentation = true
	app.navigation.Overlays.Push(overlay, FocusTarget{}, nil)
}

func setTestAppProjects(app *AppModel, projects ...*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	app.projects = make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		catalog.UpsertProject(project)
		app.projects[project.ID] = project
	}
	app.runtime.SetCatalogInMemory(catalog)
}

func addTestAppProject(app *AppModel, project *models.Project) {
	projects := append(app.allProjects(), project)
	setTestAppProjects(app, projects...)
}

func setTestCurrentProject(app *AppModel, projectID string) {
	machine := cloneMachineState(app.runtime.Machine())
	machine.CurrentProject = projectID
	app.runtime.SetMachineInMemory(machine)
	app.currentProject = app.projectByID(projectID)
}
