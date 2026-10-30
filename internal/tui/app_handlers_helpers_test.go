package tui

import (
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

func sendAppKey(app *AppModel, key tea.KeyPressMsg) *AppModel {
	model, _ := app.Update(key)
	return model.(*AppModel)
}

// newTestAppModel creates an AppModel for testing with a temp config file.
func newTestAppModel() *AppModel {
	cfg := newTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	runtime := testRuntimeAt(cfg.DataDir(), cfg)

	return &AppModel{
		domain:  newDomainController(runtime, manager),
		viewNav: newViewNavigationController(),
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

	domain := newDomainController(runtime, manager)
	domain.projects = map[string]*models.Project{project.ID: project}
	app := &AppModel{
		domain:         domain,
		viewNav:        newViewNavigationController(),
		currentProject: project,
	}

	return app, project
}

// setTestPresentationPath installs renderer identities through the same typed
// route/overlay authority as production: an overlay if the presentation has an
// overlay class, otherwise the typed Manage child the collection surfaces are
// only ever reached through.
func setTestPresentationPath(app *AppModel, views ...presentation.PresentationID) {
	app.viewNav.navigation = route.NewNavigationState()
	for _, view := range views {
		if view == presentation.ViewUnknown {
			continue
		}
		if view == presentation.ViewCrossProjectOutliner {
			app.viewNav.navigation.ResetRoot()
			continue
		}
		if app.installTypedPresentation(view) {
			continue
		}
		if err := app.viewNav.navigation.PushChild(manageRouteForPresentation(view)); err != nil {
			panic(err)
		}
	}
}

func pushTestPresentationOverlay(app *AppModel, view presentation.PresentationID) {
	if !app.installTypedPresentation(view) {
		panic("no typed overlay carries presentation " + view.String())
	}
}

// manageRouteForPresentation rebuilds the typed Manage route behind a
// collection renderer. A fixture project ID suffices: with no live hub the
// shell derives the presentation from the params alone, so the route stays the
// authority even for shells assembled without a runtime.
func manageRouteForPresentation(view presentation.PresentationID) route.Route {
	params := route.ManageParams{Module: route.ManageNotes}
	switch view {
	case presentation.ViewGlobalNotes:
		params.NotesScope = route.GlobalNotesScope()
	case presentation.ViewNotes:
		params.NotesScope = route.ProjectNotesScope("test-notes-scope")
	case presentation.ViewProjectHub:
		params = route.ManageParams{Module: route.ManageProjects}
	default:
		panic("no typed route installs presentation " + view.String())
	}
	target, err := route.NewRoute(route.SecondaryTool, params)
	if err != nil {
		panic(err)
	}
	return target
}

func setTestAppProjects(app *AppModel, projects ...*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	app.domain.projects = make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
		app.domain.projects[project.ID] = project
	}
	app.domain.runtime.SetCatalogInMemory(catalog)
}

func addTestAppProject(app *AppModel, project *models.Project) {
	projects := append(app.projectsInRuntimeOrder(app.domain.runtime.AllProjects()), project)
	setTestAppProjects(app, projects...)
}
