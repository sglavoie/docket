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
	cfg := newTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	return &AppModel{
		configManager: manager,
		runtime:       runtime,
		deps:          newShellDeps(nil),
		projects:      make(map[string]*models.Project),
		persister:     NewProjectService(),
		navigation:    NewNavigationState(),
		width:         80,
		height:        24,
		views:         NewViewRegistry(),
	}
}

// newTestAppModelWithProject creates an AppModel with a test project and temp config.
func newTestAppModelWithProject() (*AppModel, *models.Project) {
	cfg := newTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	setTestProjects(cfg, []*models.Project{project})

	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	app := &AppModel{
		configManager:  manager,
		runtime:        runtime,
		deps:           newShellDeps(nil),
		projects:       map[string]*models.Project{project.ID: project},
		persister:      NewProjectService(),
		navigation:     NewNavigationState(),
		width:          80,
		height:         24,
		views:          NewViewRegistry(),
		currentProject: project,
	}

	return app, project
}

// setTestPresentationPath installs renderer identities through the same typed
// route/overlay authority as production. The fallback is only for deliberately
// partial fixtures that lack the entity data a production route requires.
func setTestPresentationPath(app *AppModel, views ...PresentationID) {
	app.navigation = NewNavigationState()
	for _, view := range views {
		if view == ViewUnknown {
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
		app.navigation.Overlays.Push(overlay, nil)
	}
}

func pushTestPresentationOverlay(app *AppModel, view PresentationID) {
	overlay, err := NewOverlay(OverlayForm, "test:"+view.String())
	if err != nil {
		panic(err)
	}
	overlay.presentation = view
	overlay.hasPresentation = true
	app.navigation.Overlays.Push(overlay, nil)
}

func setTestAppProjects(app *AppModel, projects ...*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	app.projects = make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
		app.projects[project.ID] = project
	}
	app.runtime.SetCatalogInMemory(catalog)
}

func addTestAppProject(app *AppModel, project *models.Project) {
	projects := append(app.projectsInRuntimeOrder(app.runtime.AllProjects()), project)
	setTestAppProjects(app, projects...)
}
