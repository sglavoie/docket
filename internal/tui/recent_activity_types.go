package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	config *config.Config

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
func NewRecentActivityModel(cfg *config.Config) *RecentActivityModel {
	m := &RecentActivityModel{
		config:      cfg,
		FilterMixin: FilterMixin{filter: newFilterState()},
		tabs:        TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}

// NewProjectRecentActivityModel creates a project-scoped recent activity model.
func NewProjectRecentActivityModel(cfg *config.Config, project *models.Project) *RecentActivityModel {
	m := &RecentActivityModel{
		config:       cfg,
		projectScope: project,
		FilterMixin:  FilterMixin{filter: newFilterState()},
		tabs:         TabState{Names: recentActivityTabNames},
	}
	if !cfg.GetRecentActivityShowAll() {
		m.tabs.Active = recentActivityTabActive
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Filter tasks...", 100, 40)
	m.loadTasks()
	return m
}
