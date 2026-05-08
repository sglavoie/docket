package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
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
	config        *config.Config
	configManager *config.Manager

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
func NewRecentActivityModel(cfg *config.Config, mgr *config.Manager) *RecentActivityModel {
	m := &RecentActivityModel{
		config:        cfg,
		configManager: mgr,
		FilterMixin:   FilterMixin{filter: newFilterState()},
		tabs:          TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// NewProjectRecentActivityModel creates a project-scoped recent activity model.
func NewProjectRecentActivityModel(cfg *config.Config, mgr *config.Manager, project *models.Project) *RecentActivityModel {
	m := &RecentActivityModel{
		config:        cfg,
		configManager: mgr,
		projectScope:  project,
		FilterMixin:   FilterMixin{filter: newFilterState()},
		tabs:          TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}
