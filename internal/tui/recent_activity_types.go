package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/workflow"
)

var recentActivityTabNames = []string{"All", "Active", "Completed"}

const (
	recentActivityTabActive    = 1
	recentActivityTabCompleted = 2
)

// RecentActivityModel displays recently modified tasks across all projects.
type RecentActivityModel struct {
	BaseSearchableView
	InteractiveViewState
	configManager *config.Manager
	// runtime backs the domain-native scoring read used by common filters.
	runtime *config.RuntimeState

	// taskItem routes the standard single-task keymap mutations through the
	// application-layer coordinator, running the workflow cascade. The shell wires
	// it via SetTaskItemCoordinator (taskItemWireable) when the view is shown; a
	// zero value means the view was built in isolation (render-only test).
	taskItem TaskItemCoordinator

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	tasks    []CrossProjectTask
	filtered []CrossProjectTask

	tabs TabState

	// Filter
	FilterMixin

	// When non-nil, scopes to a single project (omits project badges).
	projectScope *models.Project
}

// NewRecentActivityModel creates a new recent activity model.
func NewRecentActivityModel(runtime *config.RuntimeState, mgr *config.Manager) *RecentActivityModel {
	m := &RecentActivityModel{
		configManager: mgr,
		runtime:       runtime,
		FilterMixin:   FilterMixin{filter: newFilterState()},
		tabs:          TabState{Names: recentActivityTabNames},
	}
	if !runtime.Settings().GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// NewProjectRecentActivityModel creates a project-scoped recent activity model.
func NewProjectRecentActivityModel(runtime *config.RuntimeState, mgr *config.Manager, project *models.Project) *RecentActivityModel {
	m := &RecentActivityModel{
		configManager: mgr,
		runtime:       runtime,
		projectScope:  project,
		FilterMixin:   FilterMixin{filter: newFilterState()},
		tabs:          TabState{Names: recentActivityTabNames},
	}
	if !runtime.Settings().GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// SetTaskItemCoordinator injects the application-layer single-task coordinator so
// the shell wires it from the shared showView/replaceView site (taskItemWireable).
func (m *RecentActivityModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// taskItemCoordinator exposes the single-task coordinator to the shared
// workflow-aware secondary-view helpers (workflowOpsView).
func (m *RecentActivityModel) taskItemCoordinator() TaskItemCoordinator { return m.taskItem }

// workflowRules is the workflowOpsView accessor for the enabled workflow rules;
// it returns nil when no config is bound (isolated render tests).
func (m *RecentActivityModel) workflowRules() []workflow.WorkflowRule {
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Automation().GetEnabledWorkflowRules()
}

func (m *RecentActivityModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *RecentActivityModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.resetViewScrollOffset()
}
