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

	// View models
	dashboard          *DashboardModel
	projectList        *ProjectListModel
	projectTree        *ProjectTreeModel
	taskDetail         *TaskDetailModel
	projectCreator     *ProjectCreatorModel
	projectEditor      *ProjectEditorModel
	taskCreator        *TaskCreatorModel
	taskEditor         *TaskEditorModel
	settings           *SettingsModel
	helpView           *HelpViewModel
	dateInput          *DateInputModel
	dependencySelector *DependencySelectorModel
	filterView         *FilterViewModel
	taskJumper         *TaskJumperModel
	projectPreview     *ProjectPreviewModel
	dueDateOverview    *DueDateOverviewModel
	tagManager         *TagManagerModel
	globalTagManager   *GlobalTagManagerModel
	tagFilterSelector  *TagFilterSelectorModel
	contextPicker      *ContextPickerModel

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
	}

	// Initialize dashboard
	app.dashboard = NewDashboardModel(cfg)

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
