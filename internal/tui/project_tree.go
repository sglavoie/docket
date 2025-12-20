package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SortField represents the field to sort tasks by.
type SortField int

const (
	SortByDefault   SortField = iota // existing order (by type hierarchy)
	SortByPriority                   // high priority first (1 = highest)
	SortByDueDate                    // earliest due date first
	SortByCreatedAt                  // by creation date
	SortByStatus                     // by workflow status order
)

// DueDateFilter represents the due date filter options.
type DueDateFilter int

const (
	DueDateAll      DueDateFilter = iota // show all tasks
	DueDateOverdue                       // only overdue tasks
	DueDateToday                         // tasks due today
	DueDateThisWeek                      // tasks due within 7 days
	DueDateNone                          // tasks with no due date
)

// String returns the display name for a sort field.
func (s SortField) String() string {
	switch s {
	case SortByPriority:
		return "priority"
	case SortByDueDate:
		return "due date"
	case SortByCreatedAt:
		return "created"
	case SortByStatus:
		return "status"
	default:
		return "default"
	}
}

// treeRefreshMsg signals the tree should rebuild itself.
// This is an internal message that preserves filter state.
type treeRefreshMsg struct{}

// TreeNode represents a node in the task tree.
type TreeNode struct {
	Task     *models.Task
	Level    int
	Expanded bool
	Visible  bool
}

// ProjectTreeModel represents the project tree view.
type ProjectTreeModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager
	nodes         []*TreeNode
	cursor        int
	width         int
	height        int

	// Filter options
	showCompleted  bool
	filterType     models.TaskType
	filterStatus   models.TaskStatus // filter by specific status
	filterPriority int               // 0=all, 1=high only (<=2)
	filterTags     map[string]bool   // filter by tag names (AND logic)
	filterDueDate  DueDateFilter     // filter by due date

	// Sort options
	sortField     SortField // current sort field
	sortAscending bool      // sort direction

	// Search state
	searchMode  bool
	searchInput textinput.Model
	searchQuery string

	// Confirmation state for destructive actions
	confirmAction string       // "", "archive", "unarchive", "delete", "bulk-delete"
	deleteTarget  *models.Task // Task to delete when confirmAction="delete"

	// Multi-select state
	multiSelectMode bool
	selectedTasks   map[string]bool // task IDs currently selected

	// Tag input for bulk tagging
	tagInputMode bool
	tagInput     textinput.Model
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

	tagInput := textinput.New()
	tagInput.Placeholder = "Enter tag name..."
	tagInput.CharLimit = 50
	tagInput.Width = 30

	m := &ProjectTreeModel{
		project:        project,
		config:         cfg,
		configManager:  manager,
		cursor:         0,
		showCompleted:  false,
		filterType:     "",
		filterStatus:   "",
		filterPriority: 0,
		filterTags:     make(map[string]bool),
		sortField:      SortByDefault,
		sortAscending:  true,
		searchMode:     false,
		searchInput:    searchInput,
		searchQuery:    "",
		selectedTasks:  make(map[string]bool),
		tagInput:       tagInput,
	}
	m.buildTree()
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.searchMode || m.searchQuery != ""
}

// clampCursor ensures the cursor is within valid bounds.
func (m *ProjectTreeModel) clampCursor() {
	visible := m.visibleNodes()
	if m.cursor >= len(visible) {
		m.cursor = len(visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// SetSize sets the view dimensions.
func (m *ProjectTreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ProjectTreeModel) Init() tea.Cmd {
	return nil
}
