package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewProjectCreator
	ViewProjectEditor
	ViewTaskCreator
	ViewTaskEditor
	ViewNoteCreator
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewDependencyViewer
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
	ViewGlobalSearch
	ViewParentSelector
	ViewPriorityView
	ViewQuickCapture
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
	dashboard    *DashboardModel
	projectTree  *ProjectTreeModel
	helpView     *HelpViewModel
	taskJumper   *TaskJumperModel
	globalSearch *GlobalSearchModel

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
// Preserves filter/sort state across the refresh.
func (m *AppModel) refreshProjectTree() {
	if m.currentProject == nil {
		return
	}
	// Save focus state (transient, not persisted to config)
	var focusedTaskID string
	if m.projectTree != nil && m.projectTree.focusedTask != nil {
		focusedTaskID = m.projectTree.focusedTask.ID
	}

	m.saveProjectTreeState()
	m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
	m.restoreProjectTreeState()

	// Restore focus state if the task still exists
	if focusedTaskID != "" {
		if task := m.currentProject.GetTaskByID(focusedTaskID); task != nil {
			m.projectTree.focusedTask = task
			m.projectTree.buildTree()
		}
	}
}

// refreshProjectTreeWith recreates the project tree view with the given project.
// Preserves filter/sort state across the refresh.
func (m *AppModel) refreshProjectTreeWith(project *models.Project) {
	// Save focus state (transient, not persisted to config)
	var focusedTaskID string
	if m.projectTree != nil && m.projectTree.focusedTask != nil {
		focusedTaskID = m.projectTree.focusedTask.ID
	}

	m.saveProjectTreeState()
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
	m.restoreProjectTreeState()

	// Restore focus state if the task still exists
	if focusedTaskID != "" {
		if task := project.GetTaskByID(focusedTaskID); task != nil {
			m.projectTree.focusedTask = task
			m.projectTree.buildTree()
		}
	}
}

// showView is a helper that registers a view, sets its size, and pushes it onto the view stack.
// This consolidates the common pattern used by Show*Msg handlers.
func (m *AppModel) showView(viewType ViewType, view View) {
	view.SetSize(m.width, m.height-2)
	m.views.Register(viewType, view)
	m.pushView(viewType)
}

// saveProjectTreeState persists the current project tree filter/sort state to config.
func (m *AppModel) saveProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}

	var filterTags []string
	for tag := range m.projectTree.filterTags {
		filterTags = append(filterTags, tag)
	}
	var filterSections []string
	for section := range m.projectTree.filterSections {
		filterSections = append(filterSections, section)
	}

	state := config.ProjectViewState{
		FilterType:     string(m.projectTree.filterType),
		FilterStatus:   string(m.projectTree.filterStatus),
		FilterPriority: m.projectTree.filterPriority,
		FilterTags:     filterTags,
		FilterSections: filterSections,
		FilterDueDate:  int(m.projectTree.filterDueDate),
		SortField:      int(m.projectTree.sortField),
		SortAscending:  m.projectTree.sortAscending,
		ShowCompleted:  m.projectTree.showCompleted,
	}
	m.config.SetProjectViewState(m.currentProject.ID, state)
	m.configManager.Save(m.config)
}

// switchToProject handles switching to a different project, loading it and creating the tree view.
// It saves current state, loads the project from storage, applies colors, and creates the tree.
// Pass restoreState=false to skip restoring saved filter/sort state (e.g., when applying custom filters).
func (m *AppModel) switchToProject(project *models.Project, restoreState bool) {
	m.saveProjectTreeState()
	m.currentProject = project
	storage.LoadProject(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	if restoreState {
		m.restoreProjectTreeState()
	}
	m.views.Register(ViewProjectTree, m.projectTree)
}

// restoreProjectTreeState applies saved filter/sort state to the current project tree.
func (m *AppModel) restoreProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}
	state, exists := m.config.GetProjectViewState(m.currentProject.ID)
	if !exists {
		return
	}

	m.projectTree.filterType = models.TaskType(state.FilterType)
	m.projectTree.filterStatus = models.TaskStatus(state.FilterStatus)
	m.projectTree.filterPriority = state.FilterPriority
	m.projectTree.filterDueDate = DueDateFilter(state.FilterDueDate)
	m.projectTree.sortField = SortField(state.SortField)
	m.projectTree.sortAscending = state.SortAscending
	m.projectTree.showCompleted = state.ShowCompleted

	// Restore tags - validate they still exist
	m.projectTree.filterTags = make(map[string]bool)
	existingTags := m.projectTree.getAllTags()
	tagSet := make(map[string]bool)
	for _, t := range existingTags {
		tagSet[t] = true
	}
	for _, tag := range state.FilterTags {
		if tagSet[tag] {
			m.projectTree.filterTags[tag] = true
		}
	}

	// Restore sections - validate they still exist
	m.projectTree.filterSections = make(map[string]bool)
	existingSections := m.currentProject.GetSections()
	sectionSet := make(map[string]bool)
	for _, s := range existingSections {
		sectionSet[s] = true
	}
	for _, section := range state.FilterSections {
		if sectionSet[section] {
			m.projectTree.filterSections[section] = true
		}
	}

	m.projectTree.updateVisibility()
	m.projectTree.clampCursor()
}

// errorCmd creates a tea.Cmd that returns an ErrorMsg with a formatted error.
func errorCmd(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: fmt.Errorf(format, args...)}
	}
}
