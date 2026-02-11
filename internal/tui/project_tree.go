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
	SortByScore                      // by composite scoring profile score (highest first)
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
	ViewModeReview                    // Only Review section tasks
	ViewModeSnoozed                   // Only snoozed tasks
	ViewModeAll                       // All non-completed, non-snoozed tasks from all sections
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
	case ViewModeReview:
		return "Review"
	case ViewModeCompleted:
		return "Completed"
	case ViewModeSnoozed:
		return "Snoozed"
	case ViewModeAll:
		return "All"
	default:
		return "Active"
	}
}

// String returns the display name for a sort field.
// ShortString returns an abbreviated label for compact display (e.g. column headers).
func (s SortField) ShortString() string {
	switch s {
	case SortByPriority:
		return "pri"
	case SortByDueDate:
		return "due"
	case SortByCreatedAt:
		return "new"
	case SortByStatus:
		return "sts"
	case SortByScore:
		return "scr"
	default:
		return ""
	}
}

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
	case SortByScore:
		return "score"
	default:
		return "default"
	}
}

// treeRefreshMsg signals the tree should rebuild itself.
// This is an internal message that preserves filter state.
type treeRefreshMsg struct{}

// FilterState contains all task filtering options.
type FilterState struct {
	CommonFilterState                 // embedded common filter fields
	Sections          map[string]bool // filter by section names (OR logic)
	Actionable        bool            // only show tasks ready to work on (no blockers, not in Inbox/Backlog)
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

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // ",", "z", or "g" when waiting for second key

	// Multi-select state
	multiSelectMode bool
	selectedTasks   map[string]bool // task IDs currently selected

	// Components
	quickEdit *QuickEditComponent
	filterBar *FilterBarComponent
	bulkOp    *BulkOperationComponent
	sidebar   *SidebarComponent

	// Custom section tab (0 = "All", 1+ = custom section index)
	customSectionTab int

	// Focus mode state
	focusedTask *models.Task // When non-nil, only show this task and descendants

	// Score filter custom value input
	scoreFilterInputMode bool            // True when entering custom score threshold
	scoreFilterInput     textinput.Model // Text input for custom score value

	// Undo context for the next save operation
	pendingUndoAction  string // Action description (e.g., "Complete Task")
	pendingUndoContext string // Additional context (e.g., task title)

	// Active saved view name (set when a saved view is loaded, cleared on manual filter/sort changes)
	activeSavedView string
}

// clearActiveSavedView clears the active saved view indicator.
// Called when the user manually changes filter, sort, or view mode.
func (m *ProjectTreeModel) clearActiveSavedView() {
	m.activeSavedView = ""
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	m := &ProjectTreeModel{
		project:        project,
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		cursor:         0,
		filter: FilterState{
			CommonFilterState: CommonFilterState{
				Tags:  make(map[string]bool),
				Attrs: make(map[string]string),
			},
			Sections: make(map[string]bool),
		},
		sortField:     SortByDefault,
		sortAscending: true,
		confirmAction: ConfirmNone,
		selectedTasks: make(map[string]bool),
		quickEdit:     NewQuickEditComponent(),
		filterBar:     NewFilterBarComponent(),
		bulkOp:        NewBulkOperationComponent(),
		sidebar:       NewSidebarComponent(),
	}
	m.InitSearchInput("Search tasks...", 100, 40)
	m.buildTree()
	return m
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *ProjectTreeModel) customSectionTabs() []string {
	custom := m.project.GetCustomSections()
	if len(custom) == 0 {
		return nil
	}
	tabs := make([]string, 0, len(custom)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, custom...)
	return tabs
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *ProjectTreeModel) activeCustomSectionFilter() string {
	tabs := m.customSectionTabs()
	if tabs == nil || m.customSectionTab <= 0 || m.customSectionTab >= len(tabs) {
		return ""
	}
	return tabs[m.customSectionTab]
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when the project tree has an active text input.
func (m *ProjectTreeModel) HasTextInput() bool {
	return m.HasActiveSearch() || m.quickEdit.IsActive() || m.scoreFilterInputMode
}

// ShouldCaptureEsc returns true when the tree needs esc/q for internal modes.
func (m *ProjectTreeModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch() || m.IsFocusMode() || m.sidebar.IsFocused() || m.multiSelectMode || m.pendingPrefix != "" || m.quickEdit.IsActive() || m.scoreFilterInputMode
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
