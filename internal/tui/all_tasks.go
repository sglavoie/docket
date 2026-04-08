package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// AllTasksGroupBy represents the grouping strategy for the all tasks view.
type AllTasksGroupBy int

const (
	ATGroupByProject  AllTasksGroupBy = iota
	ATGroupByStatus                   // by workflow status
	ATGroupByPriority                 // by priority level
	ATGroupBySection                  // by GTD section
	ATGroupByDueDate                  // by due date bucket
	ATGroupByFlat                     // no grouping
)

func (g AllTasksGroupBy) String() string {
	switch g {
	case ATGroupByProject:
		return "Project"
	case ATGroupByStatus:
		return "Status"
	case ATGroupByPriority:
		return "Priority"
	case ATGroupBySection:
		return "Section"
	case ATGroupByDueDate:
		return "Due Date"
	case ATGroupByFlat:
		return "Flat"
	default:
		return "Project"
	}
}

// AllTasksGroup represents a named group of tasks.
type AllTasksGroup struct {
	Name  string
	Tasks []CrossProjectTask
}

// TabEntry represents a single tab in the AllTasks view, which can be either
// a built-in filter tab or a user-defined saved filter (shown as tab).
type TabEntry struct {
	Name          string
	IsBuiltIn     bool
	BuiltInID     int
	SavedFilterID string
}

// AllTasksViewModel displays all tasks across all projects.
type AllTasksViewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	// Queue indicator support
	queueIDs map[string]bool

	projectService *ProjectService
	projectCache   map[string]*models.Project

	// All loaded tasks (unfiltered)
	allTasks []CrossProjectTask

	// Grouped/filtered view
	groups     []AllTasksGroup
	totalItems int

	// Grouping
	groupBy         AllTasksGroupBy
	collapsedGroups map[string]bool

	// Tabs
	tabs       TabState
	tabEntries []TabEntry

	// Filter
	FilterMixin
	projectFilter map[string]bool // included project IDs (empty = all)

	// Sort
	sort SortState

	// Multi-select
	MultiSelectState

	// Navigation
	scrollOffset int
	groupStarts  []int // cursor positions of group headers

	// Inline preview
	previewMode bool
}

// NewAllTasksViewModel creates a new all tasks view model.
func NewAllTasksViewModel(cfg *config.Config, mgr *config.Manager) *AllTasksViewModel {
	m := &AllTasksViewModel{
		ViewBundle:       ViewBundle{config: cfg, configManager: mgr},
		projectService:   NewProjectService(),
		projectCache:     make(map[string]*models.Project),
		collapsedGroups:  make(map[string]bool),
		projectFilter:    make(map[string]bool),
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		sort:             NewSortState(),
		FilterMixin:      FilterMixin{filter: newFilterState()},
	}
	m.buildTabEntries()
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.loadAllTasks()
	return m
}

func (m *AllTasksViewModel) StatusBarLabel() string { return "All Tasks" }
