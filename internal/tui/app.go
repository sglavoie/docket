package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewTaskDetail
	ViewProjectCreator
	ViewProjectEditor
	ViewTaskCreator
	ViewTaskEditor
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewFilter
	ViewTaskJumper
	ViewProjectPreview
	ViewDueDateOverview
	ViewTagManager
	ViewGlobalTagManager
	ViewTagFilterSelector
	ViewSectionFilterSelector
	ViewSectionPicker
	ViewSectionManager
	ViewContextPicker
)

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

	// View stack for navigation
	viewStack []ViewType
	width     int
	height    int

	// View registry for centralized view management
	views *ViewRegistry

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	dashboard   *DashboardModel
	projectTree *ProjectTreeModel
	taskDetail  *TaskDetailModel
	helpView    *HelpViewModel
	taskJumper  *TaskJumperModel

	// Current project context
	currentProject *models.Project

	// State
	err           error
	quitting      bool
	statusMessage string // Temporary status message (auto-clears after timeout)
}

// NewAppModel creates a new TUI application model.
func NewAppModel(cfg *config.Config, manager *config.Manager) *AppModel {
	app := &AppModel{
		config:        cfg,
		configManager: manager,
		viewStack:     []ViewType{ViewDashboard},
		width:         80,
		height:        24,
		views:         NewViewRegistry(),
	}

	// Initialize dashboard and register it
	app.dashboard = NewDashboardModel(cfg)
	app.views.Register(ViewDashboard, app.dashboard)

	// Load current project if set
	if current := cfg.GetCurrentProject(); current != nil {
		app.currentProject = current
	}

	return app
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

// currentView returns the current view type.
func (m *AppModel) currentView() ViewType {
	if len(m.viewStack) == 0 {
		return ViewDashboard
	}
	return m.viewStack[len(m.viewStack)-1]
}

// pushView pushes a new view onto the stack.
func (m *AppModel) pushView(view ViewType) {
	m.viewStack = append(m.viewStack, view)
}

// popView removes the top view from the stack.
func (m *AppModel) popView() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
}

// Sizable is implemented by views that can be resized.
type Sizable interface {
	SetSize(width, height int)
}

// initView sets dimensions on a view and pushes it onto the view stack.
func (m *AppModel) initView(viewType ViewType, view Sizable) {
	view.SetSize(m.width, m.height-2)
	m.pushView(viewType)
}

// refreshProjectTree recreates the project tree view with current project.
// Does nothing if currentProject is nil.
func (m *AppModel) refreshProjectTree() {
	if m.currentProject == nil {
		return
	}
	m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
}

// refreshProjectTreeWith recreates the project tree view with the given project.
func (m *AppModel) refreshProjectTreeWith(project *models.Project) {
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
}

// showView is a helper that registers a view, sets its size, and pushes it onto the view stack.
// This consolidates the common pattern used by Show*Msg handlers.
func (m *AppModel) showView(viewType ViewType, view View) {
	view.SetSize(m.width, m.height-2)
	m.views.Register(viewType, view)
	m.pushView(viewType)
}
