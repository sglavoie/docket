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

	cfg := testutil.NewTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	return &AppModel{
		config:                   cfg,
		configManager:            manager,
		persister:                NewProjectService(),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectLastView:          make(map[string]ViewType),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		views:                    NewViewRegistry(),
		navHistory:               NewNavHistory(),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
	}
}

// newTestAppModelWithProject creates an AppModel with a test project and temp config.
func newTestAppModelWithProject() (*AppModel, *models.Project) {
	resetSessionMutations()

	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	app := &AppModel{
		config:                   cfg,
		configManager:            manager,
		persister:                NewProjectService(),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectLastView:          make(map[string]ViewType),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		views:                    NewViewRegistry(),
		currentProject:           project,
		navHistory:               NewNavHistory(),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
	}

	return app, project
}
