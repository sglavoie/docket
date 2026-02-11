package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
)

// KanbanColumnOrder defines the left-to-right display order of columns.
var KanbanColumnOrder = []string{
	KanbanSectionActive,
	KanbanSectionReview,
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
	BaseView

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
	kanbanHeight       int  // computed height for kanban pane (excludes divider)

	// Filter state for filtering tasks across all columns
	filter    KanbanFilterState
	filterBar *KanbanFilterBarComponent

	// Search mode state
	searchMode  bool
	searchInput textinput.Model

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

// NewKanbanViewModel creates a new Kanban view model for the given project.
// It initializes five columns mapped to Active/Next/Inbox/Waiting/Someday
// and loads non-completed tasks filtered by their Section field.
func NewKanbanViewModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *KanbanViewModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search tasks..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

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
		searchInput:   searchInput,
		quickAddInput: quickAddInput,
		// Sorting state
		sortField:     SortByDefault,
		sortAscending: true,
		columnSort:    make(map[string]SortField),
		// Multi-select state
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		// Quick edit component
		quickEdit: NewQuickEditComponent(),
	}

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

// setGroupBy sets the group-by mode, recomputes column definitions,
// reinitializes maps, resets cursor, and reloads tasks.
func (m *KanbanViewModel) setGroupBy(gb KanbanGroupBy) {
	m.groupBy = gb
	m.columnDefs = ColumnsForGroupBy(gb, m.Project)

	// Reinitialize maps for new columns
	m.Columns = make(map[string]*ColumnState)
	m.Tasks = make(map[string][]*models.Task)
	m.Nodes = make(map[string][]*KanbanNode)
	for _, def := range m.columnDefs {
		m.Columns[def.Key] = &ColumnState{}
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	m.ActiveColumn = 0
	m.columnScrollOffset = 0

	// If swimlane matches the new groupBy, disable it to avoid same-axis duplication
	if m.swimlaneEnabled() && m.swimlaneBy == gb {
		m.disableSwimlane()
		return // disableSwimlane calls loadTasks
	}

	// Reinitialize swimlane 2D maps for new column defs
	if m.swimlaneEnabled() {
		m.initSwimlane2DMaps()
	}

	m.loadTasks()
}

// cycleGroupBy cycles to the next group-by mode and persists the choice.
func (m *KanbanViewModel) cycleGroupBy() {
	next := KanbanGroupBy((int(m.groupBy) + 1) % KanbanGroupByCount)
	m.setGroupBy(next)

	// Persist to config
	if m.config != nil && m.Project != nil && m.configManager != nil {
		vs := m.config.ProjectViewStates[m.Project.ID]
		vs.KanbanGroupBy = int(next)
		m.config.ProjectViewStates[m.Project.ID] = vs
		_ = m.configManager.Save(m.config)
	}

	if next == GroupBySection {
		m.statusMessage = ""
	} else {
		m.statusMessage = "Grouped by: " + GroupByLabel(next)
	}
}

// taskColumnKey returns the column key for a task based on current group-by mode.
func (m *KanbanViewModel) taskColumnKey(task *models.Task) string {
	return TaskColumnKey(m.groupBy, task)
}

// columnLabel returns the display label for a column key.
func (m *KanbanViewModel) columnLabel(key string) string {
	for _, def := range m.columnDefs {
		if def.Key == key {
			return def.Label
		}
	}
	return sectionDisplayName(key)
}

// columnIndexForKey returns the index of the column with the given key, or -1 if not found.
func (m *KanbanViewModel) columnIndexForKey(key string) int {
	for i, def := range m.columnDefs {
		if def.Key == key {
			return i
		}
	}
	return -1
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

// loadTasks populates the Tasks map by filtering project tasks into columns.
// Only non-completed tasks are included. Each task is placed in the column
// matching its Section field (empty string maps to Active column).
// Filters are applied to determine which tasks are visible.
// After filtering, hierarchical nodes are built for each column.
func (m *KanbanViewModel) loadTasks() {
	// Delegate to swimlane loader when swimlanes are active
	if m.swimlaneEnabled() {
		m.loadTasksWithSwimlanes()
		return
	}

	// Reset custom section row focus if no custom sections exist
	if m.customSectionTabs() == nil {
		m.customSectionRowFocused = false
		m.customSectionTab = 0
	}

	// Clear existing tasks and nodes
	for _, def := range m.columnDefs {
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	if m.Project == nil {
		return
	}

	// Pass 1: Filter and categorize tasks by column
	for _, task := range m.Project.Tasks {
		// Handle completed tasks based on group-by mode
		if task.Status.IsComplete() {
			// In status mode, allow recently completed tasks through
			if m.groupBy == GroupByStatus && IsRecentlyCompleted(task, CompletedInStatusModeDays) {
				// Allow — will be placed in Done/Canceled column below
			} else {
				continue
			}
		}

		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}

		// Apply filters - skip tasks that don't pass
		if !m.passesFilters(task) {
			continue
		}

		// In focus mode, only include focused task and its descendants
		if m.FocusedTask != nil && !m.isInFocusedSubtree(task) {
			continue
		}

		// Determine which column this task belongs to
		colKey := m.taskColumnKey(task)
		// Validate the column key is one of our known columns
		if _, exists := m.Columns[colKey]; !exists {
			if IsSectionGroupBy(m.groupBy) {
				// Unknown section defaults to Active (empty string)
				colKey = KanbanSectionActive
			} else {
				// For other modes, skip tasks with unknown column keys
				continue
			}
		}

		m.Tasks[colKey] = append(m.Tasks[colKey], task)
	}

	// Pass 2: Build hierarchical nodes for each column
	for _, def := range m.columnDefs {
		m.buildColumnNodes(def.Key)
		// Apply sorting after building nodes
		m.sortColumnNodes(def.Key)
	}

	// Pass 3: Compute ChildrenElsewhere counts
	m.computeChildrenElsewhere()

	// Update node visibility based on expand state
	m.updateNodeVisibility()

	// Validate cursor positions for each column after loading
	for _, def := range m.columnDefs {
		visibleCount := m.visibleNodeCount(def.Key)
		m.Columns[def.Key].Validate(visibleCount)
	}

	// Calculate filter counts for the filter bar
	m.calculateFilterCounts()
}

// isInFocusedSubtree checks if a task is the focused task or one of its descendants.
func (m *KanbanViewModel) isInFocusedSubtree(task *models.Task) bool {
	if m.FocusedTask == nil {
		return true
	}
	if task.ID == m.FocusedTask.ID {
		return true
	}
	// Walk up the parent chain
	current := task
	for current.Parent != nil {
		if current.Parent.ID == m.FocusedTask.ID {
			return true
		}
		current = current.Parent
	}
	return false
}

// buildColumnNodes creates hierarchical KanbanNode entries for a section.
// Root tasks (no parent or parent not in section) appear at level 0.
// Children are nested below their parents with increasing levels.
func (m *KanbanViewModel) buildColumnNodes(section string) {
	tasks := m.Tasks[section]
	if len(tasks) == 0 {
		return
	}

	// Build a set of task IDs in this section for quick lookup
	taskIDsInSection := make(map[string]bool)
	for _, task := range tasks {
		taskIDsInSection[task.ID] = true
	}

	// Identify root tasks: no parent, or parent not in this section
	var rootTasks []*models.Task
	for _, task := range tasks {
		isRoot := task.ParentID == nil || !taskIDsInSection[*task.ParentID]
		// In focus mode, the focused task is always a root
		if m.FocusedTask != nil && task.ID == m.FocusedTask.ID {
			isRoot = true
		}
		if isRoot {
			rootTasks = append(rootTasks, task)
		}
	}

	// Add nodes recursively starting from root tasks
	for _, root := range rootTasks {
		m.addKanbanNode(root, 0, section, taskIDsInSection)
	}
}

// addKanbanNode recursively adds a task and its children to the Nodes list.
func (m *KanbanViewModel) addKanbanNode(task *models.Task, level int, section string, taskIDsInSection map[string]bool) {
	// Cap level at MaxKanbanDisplayLevel
	displayLevel := level
	if displayLevel > MaxKanbanDisplayLevel {
		displayLevel = MaxKanbanDisplayLevel
	}

	// Check if this task has children in the same section
	hasChildrenHere := false
	for _, child := range task.Children {
		if taskIDsInSection[child.ID] {
			hasChildrenHere = true
			break
		}
	}

	// Get or default expand state (default = expanded)
	expanded := true
	if state, ok := m.ExpandState[task.ID]; ok {
		expanded = state
	} else {
		m.ExpandState[task.ID] = true
	}

	node := &KanbanNode{
		Task:            task,
		Level:           displayLevel,
		Expanded:        expanded,
		Visible:         true, // Will be updated in updateNodeVisibility
		HasChildrenHere: hasChildrenHere,
	}
	m.Nodes[section] = append(m.Nodes[section], node)

	// Recursively add children that are in this section
	for _, child := range task.Children {
		if taskIDsInSection[child.ID] {
			m.addKanbanNode(child, level+1, section, taskIDsInSection)
		}
	}
}

// computeChildrenElsewhere calculates how many children are in other columns.
func (m *KanbanViewModel) computeChildrenElsewhere() {
	// Build a map of which section each task is in
	taskSections := make(map[string]string)
	for section, tasks := range m.Tasks {
		for _, task := range tasks {
			taskSections[task.ID] = section
		}
	}

	// For each node, count children in other columns
	for _, def := range m.columnDefs {
		section := def.Key
		for _, node := range m.Nodes[section] {
			count := 0
			for _, child := range node.Task.Children {
				// Skip completed children
				if child.Status.IsComplete() {
					continue
				}
				childSection, inView := taskSections[child.ID]
				if inView && childSection != section {
					count++
				}
			}
			node.ChildrenElsewhere = count
		}
	}
}

// updateNodeVisibility sets Visible based on parent expand state.
func (m *KanbanViewModel) updateNodeVisibility() {
	for _, def := range m.columnDefs {
		section := def.Key
		nodes := m.Nodes[section]
		for i, node := range nodes {
			// Root nodes (level 0) are always visible
			if node.Level == 0 {
				node.Visible = true
				continue
			}

			// Find parent node (must be earlier in list with lower level)
			parentVisible := false
			parentExpanded := false
			for j := i - 1; j >= 0; j-- {
				if nodes[j].Level < node.Level {
					parentVisible = nodes[j].Visible
					parentExpanded = nodes[j].Expanded
					break
				}
			}

			// Visible if parent is visible AND expanded
			node.Visible = parentVisible && parentExpanded
		}
	}
}

// visibleNodeCount returns the count of visible nodes in a section.
func (m *KanbanViewModel) visibleNodeCount(section string) int {
	count := 0
	for _, node := range m.Nodes[section] {
		if node.Visible {
			count++
		}
	}
	return count
}

// visibleNodes returns only visible nodes for a section.
func (m *KanbanViewModel) visibleNodesForSection(section string) []*KanbanNode {
	var visible []*KanbanNode
	for _, node := range m.Nodes[section] {
		if node.Visible {
			visible = append(visible, node)
		}
	}
	return visible
}

// CurrentSection returns the key of the currently active column.
// In section mode this is the section name; in other modes it's the column key.
func (m *KanbanViewModel) CurrentSection() string {
	return m.activeColumnKey()
}

// CurrentColumnTasks returns the tasks in the currently active column.
func (m *KanbanViewModel) CurrentColumnTasks() []*models.Task {
	return m.Tasks[m.activeColumnKey()]
}

// TasksForSection returns the tasks in the specified section.
func (m *KanbanViewModel) TasksForSection(section string) []*models.Task {
	if tasks, exists := m.Tasks[section]; exists {
		return tasks
	}
	return nil
}

// SetSize distributes the available width across five columns and computes
// the split view heights. The view is split 50/50 vertically: top half for
// the kanban board, bottom half for task details.
func (m *KanbanViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)

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

// ValidateCursor clamps the cursor position to valid bounds for the given task count.
// For an empty column (taskCount=0), cursor is set to 0.
// For non-empty columns, cursor is clamped to [0, taskCount-1].
// Call this after task mutations that may invalidate the cursor position.
func (cs *ColumnState) ValidateCursor(taskCount int) {
	cs.Cursor = ClampCursor(cs.Cursor, taskCount)
}

// Validate adjusts Cursor and ScrollOffset to be within valid bounds for the
// given task count. Call this after any mutation that changes the column's
// task list (task moves, status changes, deletions).
func (cs *ColumnState) Validate(taskCount int) {
	cs.ValidateCursor(taskCount)
	cs.validateScrollOffset(taskCount)
}

// validateScrollOffset ensures ScrollOffset keeps the selected task visible.
// This is called internally by Validate.
func (cs *ColumnState) validateScrollOffset(taskCount int) {
	if taskCount == 0 {
		cs.ScrollOffset = 0
		return
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
	// Ensure cursor is visible (scroll offset should not exceed cursor)
	if cs.ScrollOffset > cs.Cursor {
		cs.ScrollOffset = cs.Cursor
	}
}

// InBounds returns true if the cursor is valid for the given task count.
func (cs *ColumnState) InBounds(taskCount int) bool {
	return CursorInBounds(cs.Cursor, taskCount)
}

// ValidateScroll adjusts the scroll offset to ensure the cursor row remains
// visible within the column's viewport. Call this after cursor movement or
// viewport resize to keep the selected task visible.
//
// The method ensures:
// - Cursor above viewport: scroll up so cursor is at top of viewport
// - Cursor below viewport: scroll down so cursor is at bottom of viewport
// - Scroll offset clamped to valid range [0, max(0, taskCount-viewportHeight)]
func (cs *ColumnState) ValidateScroll(taskCount, viewportHeight int) {
	if taskCount == 0 || viewportHeight <= 0 {
		cs.ScrollOffset = 0
		return
	}

	// If cursor is above the viewport, scroll up
	if cs.Cursor < cs.ScrollOffset {
		cs.ScrollOffset = cs.Cursor
	}

	// If cursor is below the viewport, scroll down
	if cs.Cursor >= cs.ScrollOffset+viewportHeight {
		cs.ScrollOffset = cs.Cursor - viewportHeight + 1
	}

	// Clamp scroll offset to valid range
	maxOffset := taskCount - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if cs.ScrollOffset > maxOffset {
		cs.ScrollOffset = maxOffset
	}
	if cs.ScrollOffset < 0 {
		cs.ScrollOffset = 0
	}
}

// AdjustScrollForViewport is an alias for ValidateScroll for backwards compatibility.
// Prefer using ValidateScroll for new code.
func (cs *ColumnState) AdjustScrollForViewport(taskCount, viewportHeight int) {
	cs.ValidateScroll(taskCount, viewportHeight)
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

	case KanbanUndoPerformedMsg:
		// Undo performed from within kanban — reload tasks and stay in kanban view
		m.loadTasks()
		// Validate all column cursors after undo
		for _, def := range m.columnDefs {
			cs := m.Columns[def.Key]
			cs.Validate(m.visibleNodeCount(def.Key))
		}
		m.statusMessage = fmt.Sprintf("Undid: %s", msg.Action)
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

// toggleCurrentExpand toggles the expand state of the selected node.
func (m *KanbanViewModel) toggleCurrentExpand() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = !node.Expanded
	m.updateNodeVisibility()
	// Validate cursor after visibility change
	section := m.CurrentSection()
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// expandCurrent expands the selected node.
func (m *KanbanViewModel) expandCurrent() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = true
	m.updateNodeVisibility()
}

// collapseCurrent collapses the selected node.
func (m *KanbanViewModel) collapseCurrent() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = false
	m.updateNodeVisibility()
	// Validate cursor after visibility change
	section := m.CurrentSection()
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// expandAllInColumn expands all nodes in the current column.
func (m *KanbanViewModel) expandAllInColumn() {
	section := m.CurrentSection()
	for _, node := range m.Nodes[section] {
		if node.HasChildrenHere {
			m.ExpandState[node.Task.ID] = true
		}
	}
	m.updateNodeVisibility()
}

// collapseAllInColumn collapses all nodes in the current column.
func (m *KanbanViewModel) collapseAllInColumn() {
	section := m.CurrentSection()
	for _, node := range m.Nodes[section] {
		if node.HasChildrenHere {
			m.ExpandState[node.Task.ID] = false
		}
	}
	m.updateNodeVisibility()
	// Validate cursor after collapsing
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// focusSubtree enters focus mode, showing only the selected task and its descendants.
func (m *KanbanViewModel) focusSubtree() {
	task := m.getSelectedTask()
	if task == nil {
		return
	}
	m.FocusedTask = task
	m.loadTasks()
	m.statusMessage = "Focus mode: " + task.Title
}

// exitFocusMode exits focus mode and shows all tasks.
func (m *KanbanViewModel) exitFocusMode() {
	m.FocusedTask = nil
	m.loadTasks()
	m.statusMessage = ""
}

// moveCursorUp moves the cursor up within the current column.
func (m *KanbanViewModel) moveCursorUp() {
	cs := m.CurrentColumnState()
	if cs.Cursor > 0 {
		cs.Cursor--
	}
}

// moveCursorDown moves the cursor down within the current column.
func (m *KanbanViewModel) moveCursorDown() {
	cs := m.CurrentColumnState()
	visibleCount := m.currentVisibleNodeCount()
	if cs.Cursor < visibleCount-1 {
		cs.Cursor++
	}
}

// positionCursorOnTask positions the cursor on the specified task in the given section.
// If the task is not found, the cursor is positioned at the first task (or 0 if empty).
func (m *KanbanViewModel) positionCursorOnTask(task *models.Task, section string) {
	visibleNodes := m.visibleNodesForSection(section)
	cs := m.Columns[section]

	// Find the task among visible nodes
	for i, node := range visibleNodes {
		if node.Task.ID == task.ID {
			cs.Cursor = i
			return
		}
	}

	// Task not found (possibly collapsed), validate cursor to be in bounds
	cs.Validate(len(visibleNodes))
}

// moveColumnLeft moves focus to the previous column on the left.
// When all columns fit on screen, empty columns are skipped (except in swimlane mode).
// When horizontal scrolling is active, moves one column at a time.
func (m *KanbanViewModel) moveColumnLeft() {
	if len(m.columnDefs) > m.maxVisibleColumns() || m.swimlaneEnabled() {
		if m.ActiveColumn > 0 {
			m.ActiveColumn--
		}
	} else {
		m.ActiveColumn = m.findNextNonEmptyColumn(-1)
	}
	m.ensureActiveColumnVisible()
}

// moveColumnRight moves focus to the next column on the right.
// When all columns fit on screen, empty columns are skipped (except in swimlane mode).
// When horizontal scrolling is active, moves one column at a time.
func (m *KanbanViewModel) moveColumnRight() {
	if len(m.columnDefs) > m.maxVisibleColumns() || m.swimlaneEnabled() {
		if m.ActiveColumn < len(m.columnDefs)-1 {
			m.ActiveColumn++
		}
	} else {
		m.ActiveColumn = m.findNextNonEmptyColumn(+1)
	}
	m.ensureActiveColumnVisible()
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *KanbanViewModel) customSectionTabs() []string {
	if m.Project == nil {
		return nil
	}
	custom := m.Project.GetCustomSections()
	if len(custom) == 0 {
		return nil
	}
	tabs := make([]string, 0, len(custom)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, custom...)
	return tabs
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *KanbanViewModel) activeCustomSectionFilter() string {
	tabs := m.customSectionTabs()
	if tabs == nil || m.customSectionTab <= 0 || m.customSectionTab >= len(tabs) {
		return ""
	}
	return tabs[m.customSectionTab]
}

// View implements the tea.Model interface. Renders a split view with:
// - Filter bar (when enabled): filter badges and column counts
// - Top half: kanban columns with section-colored headers
// - Horizontal divider
// - Bottom half: task details panel for the selected task
// When in completed view mode, renders a full-width list of completed tasks.
