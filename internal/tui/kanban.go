package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Section name constants for Kanban columns.
// Active is represented by empty string per existing convention.
const (
	KanbanSectionActive  = ""
	KanbanSectionNext    = "Next"
	KanbanSectionInbox   = "Inbox"
	KanbanSectionWaiting = "Waiting"
	KanbanSectionSomeday = "Someday"
	KanbanSectionReview  = "Review"
	KanbanSectionStandby = "Standby"
)

// KanbanColumnOrder defines the left-to-right display order of columns.
var KanbanColumnOrder = []string{
	KanbanSectionActive,
	KanbanSectionReview,
	KanbanSectionStandby,
	KanbanSectionNext,
	KanbanSectionInbox,
	KanbanSectionWaiting,
	KanbanSectionSomeday,
}

// ColumnState encapsulates the cursor position and scroll offset for a single
// Kanban column. Each column maintains independent state since columns have
// different task counts.
type ColumnState struct {
	Cursor       int
	ScrollOffset int
}

// KanbanNode represents a task in the kanban view with hierarchy information.
// This is similar to TreeNode in project_tree.go but optimized for kanban display.
type KanbanNode struct {
	Task              *models.Task
	Level             int  // Nesting depth (0 = root, 1 = child, 2 = grandchild)
	Expanded          bool // Whether children in this column are visible
	Visible           bool // Whether this node should be rendered
	HasChildrenHere   bool // Has children in this same column
	ChildrenElsewhere int  // Count of children in other columns
}

// MaxKanbanDisplayLevel is the maximum nesting depth shown in kanban view.
// Deeper levels are flattened under their grandparent.
const MaxKanbanDisplayLevel = 2

// Minimum width for a Kanban column to remain legible.
// This ensures column headers and truncated task titles are readable.
const KanbanMinColumnWidth = 22

// KanbanViewModel manages the Kanban board view state.
// It displays non-completed tasks organized into five columns by section.
type KanbanViewModel struct {
	BaseSearchableView

	// Project being displayed
	Project *models.Project

	// Config for workflow rule evaluation
	config        *config.Config
	configManager *config.Manager

	// projectTree is a reference to the project tree model for filter overlay support.
	projectTree *ProjectTreeModel

	// Columns maps section name to column state.
	// Keys: "" (Active), "Next", "Inbox", "Waiting", "Someday"
	Columns map[string]*ColumnState

	// Tasks maps section name to the list of non-completed tasks in that section.
	// This is populated during construction and should be refreshed when tasks change.
	Tasks map[string][]*models.Task

	// Nodes maps section name to hierarchical node list for rendering.
	// This provides nesting information for each task in the column.
	Nodes map[string][]*KanbanNode

	// ExpandState tracks expand/collapse state per task ID.
	// Default is true (expanded). Resets when kanban view opens.
	ExpandState map[string]bool

	// FocusedTask is the root of the currently focused subtree (nil = no focus).
	// When set, only this task and its descendants are shown.
	FocusedTask *models.Task

	// Group-by mode for organizing columns
	groupBy    KanbanGroupBy
	columnDefs []KanbanColumnDef

	// Swimlane mode: second grouping dimension for 2D grid layout
	swimlaneBy           KanbanGroupBy                        // -1 = off
	swimlaneDefs         []KanbanColumnDef                    // row definitions (reuses KanbanColumnDef)
	activeSwimlane       int                                  // index into swimlaneDefs
	swimlaneScrollOffset int                                  // first visible row index
	swimlaneTasks        map[string]map[string][]*models.Task // [laneKey][colKey]
	swimlaneNodes        map[string]map[string][]*KanbanNode  // [laneKey][colKey]
	swimlaneCells        map[string]map[string]*ColumnState   // [laneKey][colKey] cursor+scroll

	// ActiveColumn tracks which column is currently focused (index into columnDefs)
	ActiveColumn int

	// columnScrollOffset is the index of the first visible column when horizontal scrolling.
	columnScrollOffset int

	// ColumnWidth is the calculated width for each column after SetSize.
	// All columns share the same width for uniform appearance.
	ColumnWidth int

	// statusMessage holds a transient message to display to the user
	statusMessage string

	// Prefix key state for Vim-style two-key sequences (e.g., ",s" for status)
	pendingPrefix string

	// Split view state: top half is kanban, bottom half is task details
	detailsPaneFocused bool // which pane has focus (false=kanban, true=details)
	detailsScroll      int  // scroll offset for description in details pane
	detailsHeight      int  // computed height for details pane (for half-page scroll)
	kanbanHeight       int  // computed height for kanban pane (excludes divider)

	// Filter state for filtering tasks across all columns
	filter    KanbanFilterState
	filterBar *KanbanFilterBarComponent

	// Quick add mode state
	quickAddMode    bool
	quickAddInput   textinput.Model
	quickAddPreview *quickCaptureResult
	quickAddError   string

	// Quick add autocomplete state (reuse patterns from QuickCaptureModel)
	quickAddAcMode        int          // acModeNone, acModeTag, etc.
	quickAddSuggestions   []suggestion // Current autocomplete suggestions
	quickAddAcCursor      int          // Selected suggestion index
	quickAddAcViewportTop int          // First visible suggestion (for scrolling)
	quickAddAcTriggerPos  int          // Position of trigger character in input

	// Caches for autocomplete
	quickAddTagCache     []string
	quickAddParentCache  []*models.Task
	quickAddSectionCache []string

	// Confirmation state (actions: "delete", "archive")
	confirm      ConfirmDialog
	deleteTarget *models.Task

	// Sorting state (per-column)
	sortField     SortField            // current sort field for active column
	sortAscending bool                 // sort direction
	columnSort    map[string]SortField // per-column sort field override

	// Multi-select state
	MultiSelectState

	// Quick edit component
	quickEdit *QuickEditComponent

	// Bulk operation component for status changes with confirmation
	bulkOp *BulkOperationComponent

	// Completed view state
	completedViewMode bool
	completedNodes    []*KanbanNode
	completedCursor   int
	completedScroll   int

	// All view state (shows all tasks from all sections in one list)
	allViewMode bool
	allNodes    []*KanbanNode
	allCursor   int
	allScroll   int

	// Tab filtering
	activeTab int // Index into kanbanTabNames

	// Custom section tab filtering
	customSectionTab        int  // selected custom section tab index (0 = All)
	customSectionRowFocused bool // true when focus is on custom section row (vs preset row)

	// Additional filter state
	actionableFilter bool

	// Score filter custom value input
	scoreFilterInput ScoreFilterInput

	// Last modified task tracking
	lastModifiedTaskID string
}

// primaryProjectView interface implementation for shared key dispatch.
func (m *KanbanViewModel) getProject() *models.Project { return m.Project }
func (m *KanbanViewModel) getConfig() *config.Config   { return m.config }
func (m *KanbanViewModel) getCursorTask() *models.Task { return m.getSelectedTask() }
func (m *KanbanViewModel) reloadFromDisk() tea.Cmd {
	_ = storage.LoadProject(m.Project)
	m.loadTasks()
	m.statusMessage = "Refreshed from disk"
	return nil
}

// NewKanbanViewModel creates a new Kanban view model for the given project.
// It initializes five columns mapped to Active/Next/Inbox/Waiting/Someday
// and loads non-completed tasks filtered by their Section field.
func NewKanbanViewModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *KanbanViewModel {
	quickAddInput := textinput.New()
	quickAddInput.Placeholder = "Task title ^type !priority #tag @due /section"
	quickAddInput.CharLimit = 200
	quickAddInput.Width = 60

	// Load group-by mode from config
	var groupBy KanbanGroupBy
	if cfg != nil && project != nil {
		if vs, ok := cfg.ProjectViewStates[project.ID]; ok {
			groupBy = KanbanGroupBy(vs.KanbanGroupBy)
			if groupBy < 0 || groupBy >= KanbanGroupByCount {
				groupBy = GroupBySection
			}
		}
	}

	// Load swimlane-by mode from config
	swimlaneBy := KanbanSwimlanesOff
	if cfg != nil && project != nil {
		if vs, ok := cfg.ProjectViewStates[project.ID]; ok && vs.KanbanSwimlaneBy != nil {
			sb := KanbanGroupBy(*vs.KanbanSwimlaneBy)
			if sb >= 0 && sb < KanbanGroupBy(KanbanGroupByCount) && sb != groupBy {
				swimlaneBy = sb
			}
		}
	}

	m := &KanbanViewModel{
		Project:       project,
		config:        cfg,
		configManager: cfgManager,
		groupBy:       groupBy,
		swimlaneBy:    swimlaneBy,
		Columns:       make(map[string]*ColumnState),
		Tasks:         make(map[string][]*models.Task),
		Nodes:         make(map[string][]*KanbanNode),
		ExpandState:   make(map[string]bool),
		filter: KanbanFilterState{
			CommonFilterState: CommonFilterState{
				Tags:  make(map[string]bool),
				Attrs: make(map[string]string),
			},
		},
		filterBar:     NewKanbanFilterBarComponent(),
		quickAddInput: quickAddInput,
		// Sorting state
		sortField:     SortByDefault,
		sortAscending: true,
		columnSort:    make(map[string]SortField),
		// Multi-select state
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		// Quick edit component
		quickEdit: NewQuickEditComponent(),
		// Bulk operation component
		bulkOp: NewBulkOperationComponent(),
	}

	m.InitSearchInput("Search tasks...", 100, 40)

	// Compute column definitions from group-by mode
	m.columnDefs = ColumnsForGroupBy(m.groupBy, project)

	// Initialize column state for each column
	for _, def := range m.columnDefs {
		m.Columns[def.Key] = &ColumnState{}
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	// Initialize swimlane if enabled
	if m.swimlaneBy != KanbanSwimlanesOff {
		m.swimlaneDefs = ColumnsForGroupBy(m.swimlaneBy, project)
		m.initSwimlane2DMaps()
	}

	// Load tasks into columns by section, excluding completed tasks
	m.loadTasks()

	return m
}

// columnKeys returns the keys from columnDefs, replacing all KanbanColumnOrder references.
func (m *KanbanViewModel) columnKeys() []string {
	keys := make([]string, len(m.columnDefs))
	for i, def := range m.columnDefs {
		keys[i] = def.Key
	}
	return keys
}

// activeColumnKey returns the key of the currently active column.
func (m *KanbanViewModel) activeColumnKey() string {
	if m.ActiveColumn >= 0 && m.ActiveColumn < len(m.columnDefs) {
		return m.columnDefs[m.ActiveColumn].Key
	}
	return ""
}

// maxVisibleColumns returns the effective max visible columns from config,
// clamped by terminal width (columns never narrower than KanbanMinColumnWidth)
// and total column count.
func (m *KanbanViewModel) maxVisibleColumns() int {
	maxCols := m.config.GetKanbanMaxColumns()

	// Clamp by terminal width so columns never go below KanbanMinColumnWidth
	if m.Width() > 0 {
		fitCols := m.Width() / KanbanMinColumnWidth
		if fitCols < 1 {
			fitCols = 1
		}
		if maxCols > fitCols {
			maxCols = fitCols
		}
	}

	// Cap at total number of column definitions
	if maxCols > len(m.columnDefs) {
		maxCols = len(m.columnDefs)
	}

	if maxCols < 1 {
		maxCols = 1
	}
	return maxCols
}

// maxVisibleSwimlanes returns the effective max visible swimlane rows from config,
// capped at total swimlane count.
func (m *KanbanViewModel) maxVisibleSwimlanes() int {
	maxRows := m.config.GetKanbanMaxSwimlaneRows()

	// Cap at total number of swimlane definitions
	if maxRows > len(m.swimlaneDefs) {
		maxRows = len(m.swimlaneDefs)
	}

	if maxRows < 1 {
		maxRows = 1
	}
	return maxRows
}

// visibleColumnRange returns the start (inclusive) and end (exclusive) indices
// of columns that should be rendered, based on the scroll offset.
func (m *KanbanViewModel) visibleColumnRange() (int, int) {
	total := len(m.columnDefs)
	maxVisible := m.maxVisibleColumns()
	if total <= maxVisible {
		return 0, total
	}
	start := m.columnScrollOffset
	if start < 0 {
		start = 0
	}
	if start > total-maxVisible {
		start = total - maxVisible
	}
	end := start + maxVisible
	if end > total {
		end = total
	}
	return start, end
}

// ensureActiveColumnVisible adjusts columnScrollOffset so that ActiveColumn
// is within the visible range.
func (m *KanbanViewModel) ensureActiveColumnVisible() {
	total := len(m.columnDefs)
	maxVisible := m.maxVisibleColumns()
	if total <= maxVisible {
		m.columnScrollOffset = 0
		return
	}
	if m.ActiveColumn < m.columnScrollOffset {
		m.columnScrollOffset = m.ActiveColumn
	}
	if m.ActiveColumn >= m.columnScrollOffset+maxVisible {
		m.columnScrollOffset = m.ActiveColumn - maxVisible + 1
	}
	// Clamp
	if m.columnScrollOffset < 0 {
		m.columnScrollOffset = 0
	}
	if m.columnScrollOffset > total-maxVisible {
		m.columnScrollOffset = total - maxVisible
	}
}

// SetSize distributes the available width across five columns and computes
// the split view heights. The view is split 50/50 vertically: top half for
// the kanban board, bottom half for task details.
func (m *KanbanViewModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)

	// Use at most maxVisibleColumns for width calculation
	numColumns := len(m.columnDefs)
	maxVisCols := m.maxVisibleColumns()
	if numColumns > maxVisCols {
		numColumns = maxVisCols
	}
	numGaps := numColumns - 1
	availableWidth := width - numGaps

	// Distribute width evenly across visible columns
	columnWidth := availableWidth / numColumns

	// Enforce minimum width to keep columns legible
	if columnWidth < KanbanMinColumnWidth {
		columnWidth = KanbanMinColumnWidth
	}

	m.ColumnWidth = columnWidth

	tabBarHeight := TabBarHeight
	if m.customSectionTabs() != nil && IsSectionGroupBy(m.groupBy) {
		tabBarHeight += TabBarHeight
	}
	if !IsSectionGroupBy(m.groupBy) {
		tabBarHeight++ // Group-by indicator line
	}
	if len(m.columnDefs) > maxVisCols {
		tabBarHeight++ // Horizontal scroll position line
	}

	// Compute split view heights: 50/50 split minus divider
	availableHeight := height - DividerHeight - tabBarHeight
	m.kanbanHeight = availableHeight / 2
	if m.kanbanHeight < 3 {
		m.kanbanHeight = 3 // Minimum height for header + 1 task
	}
}

// Init implements the tea.Model interface. The Kanban view does not require
// any initialization commands as state is set up during construction.
func (m *KanbanViewModel) Init() tea.Cmd {
	return nil
}

// Update implements the tea.Model interface. It handles window resize messages
// and keyboard navigation, delegating key handling to handleKeyMsg.
func (m *KanbanViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case KanbanTaskMovedMsg:
		// Task was moved to a different column - reload tasks and update cursor
		m.loadTasks()
		// Move active column to where the task landed
		m.ActiveColumn = msg.TargetColumnIndex
		m.ensureActiveColumnVisible()
		// Position cursor on the moved task in the new column
		m.positionCursorOnTask(msg.Task, msg.TargetSection)
		m.statusMessage = fmt.Sprintf("Moved to %s", m.columnLabel(msg.TargetSection))
		return m, nil

	case KanbanTaskReorderedMsg:
		// Task was reordered within its column - reload and position cursor
		m.loadTasks()
		cs := m.Columns[msg.Section]
		cs.Cursor = msg.NewPosition
		// Validate in case the reload changed task order
		cs.Validate(m.visibleNodeCount(msg.Section))
		return m, nil

	case KanbanStatusChangedMsg:
		// Task status changed - reload tasks since task may have moved columns
		m.loadTasks()
		// Try to position cursor on the task if it's still visible
		for colIdx, def := range m.columnDefs {
			visibleNodes := m.visibleNodesForSection(def.Key)
			for i, node := range visibleNodes {
				if node.Task.ID == msg.TaskID {
					m.ActiveColumn = colIdx
					m.ensureActiveColumnVisible()
					m.Columns[def.Key].Cursor = i
					return m, nil
				}
			}
		}
		// Task not visible (likely completed/canceled) - validate current cursor
		cs := m.CurrentColumnState()
		cs.Validate(m.currentVisibleNodeCount())
		return m, nil

	case KanbanQuickAddCreatedMsg:
		// Task was created via quick add - exit quick add mode, reload, position cursor
		m.quickAddMode = false
		m.quickAddInput.Blur()
		m.quickAddInput.SetValue("")
		m.quickAddPreview = nil
		m.quickAddError = ""
		m.loadTasks()
		// Position cursor on the newly created task
		if msg.Task != nil {
			colKey := m.taskColumnKey(msg.Task)
			for colIdx, def := range m.columnDefs {
				if def.Key == colKey {
					m.ActiveColumn = colIdx
					break
				}
			}
			m.ensureActiveColumnVisible()
			m.positionCursorOnTask(msg.Task, colKey)
		}
		m.statusMessage = fmt.Sprintf("Created: %s", msg.Task.Title)
		return m, nil

	case KanbanTaskDeletedMsg:
		// Task was deleted - reload tasks and validate cursor
		m.loadTasks()
		// Validate cursor position in the section where task was deleted
		cs := m.Columns[msg.Section]
		cs.Validate(m.visibleNodeCount(msg.Section))
		m.statusMessage = "Task deleted"
		return m, nil

	case TaskCreatedMsg:
		// Task was created via modal quick capture - reload and position cursor
		m.loadTasks()
		// Position cursor on the newly created task
		if msg.Task != nil {
			colKey := m.taskColumnKey(msg.Task)
			for colIdx, def := range m.columnDefs {
				if def.Key == colKey {
					m.ActiveColumn = colIdx
					break
				}
			}
			m.ensureActiveColumnVisible()
			m.positionCursorOnTask(msg.Task, colKey)
		}
		return m, nil
	}

	return m, nil
}

// handleKeyMsg processes keyboard input for the Kanban view.

// View implements the tea.Model interface. Renders a split view with:
// - Filter bar (when enabled): filter badges and column counts
// - Top half: kanban columns with section-colored headers
// - Horizontal divider
// - Bottom half: task details panel for the selected task
// When in completed view mode, renders a full-width list of completed tasks.
