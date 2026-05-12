package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
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
	IsOverflow    bool // placeholder "More…" entry — not a real filter
}

// AllTasksViewModel displays all tasks across all projects.
type AllTasksViewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	projectService *ProjectService
	projectCache   map[string]*models.Project

	// All loaded tasks (unfiltered)
	allTasks            []CrossProjectTask
	loadedWithCompleted bool // Whether allTasks was loaded with IncludeCompleted

	// Grouped/filtered view
	groups     []AllTasksGroup
	totalItems int

	// Grouping
	groupBy         AllTasksGroupBy
	collapsedGroups map[string]bool

	// Tabs
	tabs                 TabState
	tabEntries           []TabEntry // visible entries (capped at overflowTabCap)
	allTabEntries        []TabEntry // full list including overflow
	activeTabIdx         int        // real index into allTabEntries
	overflowPickerOpen   bool
	overflowPickerCursor int

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

	initialTabSet bool
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
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				action := m.confirm.Action()
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						if action == "delete-smart-list" {
							model, cmd := m.handleDeleteSavedFilterTab()
							return model, cmd, true
						}
						if m.multiSelectMode && m.SelectedCount() > 0 {
							model, cmd := m.handleBulkDelete()
							return model, cmd, true
						}
						model, cmd := taskOpsDeleteTask(m)
						return model, cmd, true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
	m.loadAllTasks()
	return m
}

func (m *AllTasksViewModel) StatusBarLabel() string { return "All Tasks" }

// IsFullHeight returns true because the all tasks view renders its own footer.
func (m *AllTasksViewModel) IsFullHeight() bool { return true }
