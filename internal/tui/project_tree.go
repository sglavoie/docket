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

// ViewMode represents the view mode filter for the project tree.
type ViewMode int

const (
	ViewModeActive    ViewMode = iota // Tasks not in Inbox or Backlog sections
	ViewModeInbox                     // Only Inbox section tasks
	ViewModeBacklog                   // Only Backlog section tasks
	ViewModeCompleted                 // Only completed tasks (done or cancelled)
)

// String returns the display name for a view mode.
func (v ViewMode) String() string {
	switch v {
	case ViewModeInbox:
		return "Inbox"
	case ViewModeBacklog:
		return "Backlog"
	case ViewModeCompleted:
		return "Completed"
	default:
		return "Active"
	}
}

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
	Task          *models.Task
	Level         int
	Expanded      bool
	Visible       bool
	IsContextNode bool // True if visible only to show hierarchy context (parent in different section)
}

// ProjectTreeModel represents the project tree view.
type ProjectTreeModel struct {
	BaseSearchableView
	project       *models.Project
	config        *config.Config
	configManager *config.Manager
	nodes         []*TreeNode
	cursor        int

	// View mode (Active, Inbox, or Backlog)
	viewMode ViewMode

	// Filter options
	filterType       models.TaskType
	filterStatus     models.TaskStatus // filter by specific status
	filterPriority   int               // 0=all, 1=high only (<=2)
	filterTags       map[string]bool   // filter by tag names (AND logic)
	filterSections   map[string]bool   // filter by section names (OR logic)
	filterDueDate    DueDateFilter     // filter by due date
	filterActionable bool              // only show tasks ready to work on (no blockers, not in Inbox/Backlog)

	// Sort options
	sortField     SortField // current sort field
	sortAscending bool      // sort direction

	// Confirmation state for destructive actions
	confirmAction string       // "", "archive", "unarchive", "delete", "bulk-delete"
	deleteTarget  *models.Task // Task to delete when confirmAction="delete"

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // ",", "z", or "g" when waiting for second key

	// Multi-select state
	multiSelectMode bool
	selectedTasks   map[string]bool // task IDs currently selected

	// Tag input for bulk tagging
	tagInputMode bool
	tagInput     textinput.Model

	// Focus mode state
	focusedTask *models.Task // When non-nil, only show this task and descendants

	// Sidebar state
	sidebarEnabled           bool // Toggle sidebar on/off
	sidebarFocused           bool // When true, sidebar has focus for scrolling
	sidebarScroll            int  // Scroll offset for sidebar content
	sidebarDependencyCursor  int  // Cursor position within active dependency section
	sidebarDependencySection int  // 0 = "Blocked by", 1 = "Blocking"

	// Undo context for the next save operation
	pendingUndoAction  string // Action description (e.g., "Complete Task")
	pendingUndoContext string // Additional context (e.g., task title)

	// Pending bulk status change (for confirmation prompts)
	pendingBulkStatus       models.TaskStatus       // Status to apply after confirmation
	pendingBulkTasks        []*models.Task          // Tasks to change
	pendingBulkBlockedCount int                     // Count of tasks with incomplete deps
	pendingBulkPrevStates   map[string]*models.Task // Previous states for event triggers

	// Filter bar state
	filterBarEnabled bool           // Toggle filter bar visibility
	filterCounts     map[string]int // Cached task counts per filter category

	// Children in other sections cache (computed during calculateFilterCounts)
	// Maps taskID -> sectionName -> count of descendants in that section
	childrenInOtherSections map[string]map[string]int
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	tagInput := textinput.New()
	tagInput.Placeholder = "Enter tag name..."
	tagInput.CharLimit = 50
	tagInput.Width = 30

	m := &ProjectTreeModel{
		project:                  project,
		config:                   cfg,
		configManager:            manager,
		cursor:                   0,
		filterType:               "",
		filterStatus:             "",
		filterPriority:           0,
		filterTags:               make(map[string]bool),
		filterSections:           make(map[string]bool),
		sortField:                SortByDefault,
		sortAscending:            true,
		selectedTasks:            make(map[string]bool),
		tagInput:                 tagInput,
		sidebarEnabled:           true, // Sidebar on by default
		sidebarFocused:           false,
		sidebarScroll:            0,
		sidebarDependencyCursor:  0,
		sidebarDependencySection: 0,
		filterBarEnabled:         true, // Filter bar on by default
		filterCounts:             make(map[string]int),
		childrenInOtherSections:  make(map[string]map[string]int),
	}
	m.InitSearchInput("Search tasks...", 100, 40)
	m.buildTree()
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// IsFocusMode returns true if the view is focused on a subtree.
func (m *ProjectTreeModel) IsFocusMode() bool {
	return m.focusedTask != nil
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
