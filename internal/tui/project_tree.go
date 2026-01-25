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

// ConfirmActionType represents the type of action requiring confirmation.
type ConfirmActionType int

const (
	ConfirmNone ConfirmActionType = iota
	ConfirmDelete
	ConfirmDeleteRecurring
	ConfirmBulkDelete
	ConfirmBulkStatus
	ConfirmArchiveTasks
	ConfirmArchive
	ConfirmUnarchive
	ConfirmTurnIntoNote
)

// ViewMode represents the view mode filter for the project tree.
type ViewMode int

const (
	ViewModeActive    ViewMode = iota // Tasks not in Inbox, Next, Waiting, or Someday sections
	ViewModeNext                      // Only Next section tasks (GTD "Next Actions")
	ViewModeInbox                     // Only Inbox section tasks
	ViewModeWaiting                   // Only Waiting section tasks (GTD "Waiting For")
	ViewModeSomeday                   // Only Someday section tasks (GTD "Someday/Maybe")
	ViewModeCompleted                 // Only completed tasks (done or canceled)
)

// String returns the display name for a view mode.
func (v ViewMode) String() string {
	switch v {
	case ViewModeNext:
		return "Next"
	case ViewModeInbox:
		return "Inbox"
	case ViewModeWaiting:
		return "Waiting"
	case ViewModeSomeday:
		return "Someday"
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

// FilterState contains all task filtering options.
type FilterState struct {
	Type       models.TaskType
	Status     models.TaskStatus // filter by specific status
	Priority   int               // 0=all, 1=high only (<=2)
	Tags       map[string]bool   // filter by tag names (AND logic)
	Sections   map[string]bool   // filter by section names (OR logic)
	DueDate    DueDateFilter     // filter by due date
	Actionable bool              // only show tasks ready to work on (no blockers, not in Inbox/Backlog)
}

// QuickEditState contains state for inline task editing.
type QuickEditState struct {
	Mode     bool             // True when in quick edit mode
	Rename   bool             // True if rename-only mode (no modifier parsing)
	Input    textinput.Model  // Text input for editing
	Task     *models.Task     // Task being edited
	Preview  *quickEditResult // Live preview of parsed changes
	Original *quickEditResult // Original task state for comparison
}

// BulkOperationState contains state for pending bulk operations.
type BulkOperationState struct {
	Status       models.TaskStatus       // Status to apply after confirmation
	Tasks        []*models.Task          // Tasks to change
	BlockedCount int                     // Count of tasks with incomplete deps
	PrevStates   map[string]*models.Task // Previous states for event triggers
}

// TreeNode represents a node in the task tree.
type TreeNode struct {
	Task                *models.Task
	Level               int
	Expanded            bool
	Visible             bool
	IsContextNode       bool  // True if visible only to show hierarchy context (parent in different section)
	HasFilteredChildren bool  // True if any children pass current filters (used for collapse arrow)
	MatchPositions      []int // Match positions within title for search highlighting
}

// ProjectTreeModel represents the project tree view.
type ProjectTreeModel struct {
	BaseSearchableView
	project        *models.Project
	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService
	nodes          []*TreeNode
	cursor         int

	// View mode (Active, Inbox, or Backlog)
	viewMode ViewMode

	// Filter options
	filter FilterState

	// Sort options
	sortField     SortField // current sort field
	sortAscending bool      // sort direction

	// Confirmation state for destructive actions
	confirmAction ConfirmActionType
	deleteTarget  *models.Task // Task to delete when confirmAction=ConfirmDelete
	noteTarget    *models.Task // Task to convert when confirmAction=ConfirmTurnIntoNote

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // ",", "z", or "g" when waiting for second key

	// Multi-select state
	multiSelectMode bool
	selectedTasks   map[string]bool // task IDs currently selected

	// Tag input for bulk tagging
	tagInputMode bool
	tagInput     textinput.Model

	// Components
	quickEdit *QuickEditComponent
	filterBar *FilterBarComponent
	bulkOp    *BulkOperationComponent
	sidebar   *SidebarComponent

	// Focus mode state
	focusedTask *models.Task // When non-nil, only show this task and descendants

	// Undo context for the next save operation
	pendingUndoAction  string // Action description (e.g., "Complete Task")
	pendingUndoContext string // Additional context (e.g., task title)
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	tagInput := textinput.New()
	tagInput.Placeholder = "Enter tag name..."
	tagInput.CharLimit = 50
	tagInput.Width = 30

	m := &ProjectTreeModel{
		project:        project,
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		cursor:         0,
		filter: FilterState{
			Type:       "",
			Status:     "",
			Priority:   0,
			Tags:       make(map[string]bool),
			Sections:   make(map[string]bool),
			DueDate:    DueDateAll,
			Actionable: false,
		},
		sortField:     SortByDefault,
		sortAscending: true,
		confirmAction: ConfirmNone,
		selectedTasks: make(map[string]bool),
		tagInput:      tagInput,
		quickEdit:     NewQuickEditComponent(),
		filterBar:     NewFilterBarComponent(),
		bulkOp:        NewBulkOperationComponent(),
		sidebar:       NewSidebarComponent(),
	}
	m.InitSearchInput("Search tasks...", 100, 40)
	m.buildTree()
	return m
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when the project tree search is active.
func (m *ProjectTreeModel) HasTextInput() bool {
	return m.HasActiveSearch()
}

// ShouldCaptureEsc returns true when the tree needs esc/q for internal modes.
func (m *ProjectTreeModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch() || m.IsFocusMode() || m.sidebar.IsFocused() || m.multiSelectMode || m.pendingPrefix != ""
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
