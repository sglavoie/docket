package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	ViewModeStandby                   // Only Standby section tasks
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
	case ViewModeStandby:
		return "Standby"
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
	InteractiveViewState
	project        *models.Project
	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService
	nodes          []*TreeNode

	// View mode (Active, Inbox, or Backlog)
	viewMode ViewMode

	// Filter options
	filter FilterState

	// Sort options
	sortField     SortField // current sort field
	sortAscending bool      // sort direction

	// Task to delete when confirm action is "delete" or "delete-recurring"
	deleteTarget *models.Task

	// Multi-select state
	MultiSelectState

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
	scoreFilterInput ScoreFilterInput

	// Undo context for the next save operation
	pendingUndoAction  string // Action description (e.g., "Complete Task")
	pendingUndoContext string // Additional context (e.g., task title)

	// Active saved view name (set when a saved view is loaded, cleared on manual filter/sort changes)
	activeSavedView string
}

// primaryProjectView interface implementation for shared key dispatch.
func (m *ProjectTreeModel) getProject() *models.Project { return m.project }
func (m *ProjectTreeModel) getConfig() *config.Config   { return m.config }
func (m *ProjectTreeModel) getCursorTask() *models.Task {
	visible := m.visibleNodes()
	if CursorInBounds(m.cursor, len(visible)) {
		return visible[m.cursor].Task
	}
	return nil
}
func (m *ProjectTreeModel) reloadFromDisk() tea.Cmd {
	m.clearSelection()
	_ = storage.LoadProject(m.project)
	m.buildTree()
	return nil
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
		filter: FilterState{
			CommonFilterState: CommonFilterState{
				Tags:  make(map[string]bool),
				Attrs: make(map[string]string),
			},
			Sections: make(map[string]bool),
		},
		sortField:        SortByDefault,
		sortAscending:    true,
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		quickEdit:        NewQuickEditComponent(),
		filterBar:        NewFilterBarComponent(),
		bulkOp:           NewBulkOperationComponent(),
		sidebar:          NewSidebarComponent(),
	}
	m.InitSearchInput("Search tasks...", 100, 40)
	m.buildTree()
	return m
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *ProjectTreeModel) customSectionTabs() []string {
	return CustomSectionTabs(m.project)
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *ProjectTreeModel) activeCustomSectionFilter() string {
	return ActiveCustomSectionFilter(m.project, m.customSectionTab)
}

// HasActiveSearch returns true if search mode is active or a search filter is applied.
func (m *ProjectTreeModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when the project tree has an active text input.
func (m *ProjectTreeModel) HasTextInput() bool {
	return m.HasActiveSearch() || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive()
}

// ShouldCaptureEsc returns true when the tree needs esc/q for internal modes.
func (m *ProjectTreeModel) ShouldCaptureEsc() bool {
	return m.HasActiveSearch() || m.IsFocusMode() || m.sidebar.IsFocused() || m.multiSelectMode || m.pendingPrefix != "" || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive() || m.confirm.IsActive()
}

// IsFocusMode returns true if the view is focused on a subtree.
func (m *ProjectTreeModel) IsFocusMode() bool {
	return m.focusedTask != nil
}

// clampCursor ensures the cursor is within valid bounds.
func (m *ProjectTreeModel) clampCursor() {
	m.cursor = ClampCursor(m.cursor, len(m.visibleNodes()))
}

func (m *ProjectTreeModel) Init() tea.Cmd {
	return nil
}
