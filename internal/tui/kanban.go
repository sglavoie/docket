package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
)

// KanbanColumnOrder defines the left-to-right display order of columns.
var KanbanColumnOrder = []string{
	KanbanSectionActive,
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
const KanbanMinColumnWidth = 15

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

	// ActiveColumn tracks which column is currently focused (index into KanbanColumnOrder)
	ActiveColumn int

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

	// Delete confirmation state
	confirmDelete bool
	deleteTarget  *models.Task

	// Note conversion confirmation state

	// Archive confirmation state
	confirmArchive bool

	// Sorting state (per-column)
	sortField     SortField            // current sort field for active column
	sortAscending bool                 // sort direction
	columnSort    map[string]SortField // per-column sort field override

	// Multi-select state
	multiSelectMode bool
	selectedTasks   map[string]bool // task IDs currently selected

	// Quick edit component
	quickEdit *QuickEditComponent

	// Completed view state
	completedViewMode bool
	completedNodes    []*KanbanNode
	completedCursor   int
	completedScroll   int

	// Tab filtering
	activeTab int // Index into kanbanTabNames

	// Additional filter state
	actionableFilter bool

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

	m := &KanbanViewModel{
		Project:       project,
		config:        cfg,
		configManager: cfgManager,
		Columns:       make(map[string]*ColumnState),
		Tasks:         make(map[string][]*models.Task),
		Nodes:         make(map[string][]*KanbanNode),
		ExpandState:   make(map[string]bool),
		filter: KanbanFilterState{
			Tags: make(map[string]bool),
		},
		filterBar:     NewKanbanFilterBarComponent(),
		searchInput:   searchInput,
		quickAddInput: quickAddInput,
		// Sorting state
		sortField:     SortByDefault,
		sortAscending: true,
		columnSort:    make(map[string]SortField),
		// Multi-select state
		selectedTasks: make(map[string]bool),
		// Quick edit component
		quickEdit: NewQuickEditComponent(),
	}

	// Initialize column state for each of the five sections
	for _, section := range KanbanColumnOrder {
		m.Columns[section] = &ColumnState{}
		m.Tasks[section] = []*models.Task{}
		m.Nodes[section] = []*KanbanNode{}
	}

	// Load tasks into columns by section, excluding completed tasks
	m.loadTasks()

	return m
}

// loadTasks populates the Tasks map by filtering project tasks into columns.
// Only non-completed tasks are included. Each task is placed in the column
// matching its Section field (empty string maps to Active column).
// Filters are applied to determine which tasks are visible.
// After filtering, hierarchical nodes are built for each column.
func (m *KanbanViewModel) loadTasks() {
	// Clear existing tasks and nodes
	for _, section := range KanbanColumnOrder {
		m.Tasks[section] = []*models.Task{}
		m.Nodes[section] = []*KanbanNode{}
	}

	if m.Project == nil {
		return
	}

	// Pass 1: Filter and categorize tasks by section
	for _, task := range m.Project.Tasks {
		// Skip completed tasks (done or canceled)
		if task.Status.IsComplete() {
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
		section := task.Section
		// Validate the section is one of our known columns
		if _, exists := m.Columns[section]; !exists {
			// Unknown section defaults to Active (empty string)
			section = KanbanSectionActive
		}

		m.Tasks[section] = append(m.Tasks[section], task)
	}

	// Pass 2: Build hierarchical nodes for each section
	for _, section := range KanbanColumnOrder {
		m.buildColumnNodes(section)
		// Apply sorting after building nodes
		m.sortColumnNodes(section)
	}

	// Pass 3: Compute ChildrenElsewhere counts
	m.computeChildrenElsewhere()

	// Update node visibility based on expand state
	m.updateNodeVisibility()

	// Validate cursor positions for each column after loading
	for _, section := range KanbanColumnOrder {
		visibleCount := m.visibleNodeCount(section)
		m.Columns[section].Validate(visibleCount)
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

	// For each node, count children in other sections
	for _, section := range KanbanColumnOrder {
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
	for _, section := range KanbanColumnOrder {
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

// CurrentColumnState returns the ColumnState for the currently active column.
func (m *KanbanViewModel) CurrentColumnState() *ColumnState {
	section := KanbanColumnOrder[m.ActiveColumn]
	return m.Columns[section]
}

// CurrentSection returns the section name of the currently active column.
func (m *KanbanViewModel) CurrentSection() string {
	return KanbanColumnOrder[m.ActiveColumn]
}

// CurrentColumnTasks returns the tasks in the currently active column.
func (m *KanbanViewModel) CurrentColumnTasks() []*models.Task {
	section := KanbanColumnOrder[m.ActiveColumn]
	return m.Tasks[section]
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

	// Account for gaps between columns (1 char gap between each pair)
	numColumns := len(KanbanColumnOrder)
	numGaps := numColumns - 1
	availableWidth := width - numGaps

	// Distribute width evenly across columns
	columnWidth := availableWidth / numColumns

	// Enforce minimum width to keep columns legible
	if columnWidth < KanbanMinColumnWidth {
		columnWidth = KanbanMinColumnWidth
	}

	m.ColumnWidth = columnWidth

	// Account for tab bar height (always shown, 2 lines: text + bottom border)
	tabBarHeight := 2

	// Account for filter bar height when enabled (2 lines)
	filterBarHeight := 0
	if m.filterBar != nil && m.filterBar.IsEnabled() {
		filterBarHeight = 2
	}

	// Compute split view heights: 50/50 split minus 1 for the divider line
	// kanbanHeight = (height - 1 - tabBarHeight - filterBarHeight) / 2, leaving rest for details pane
	availableHeight := height - 1 - tabBarHeight - filterBarHeight // -1 for divider
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
		// Position cursor on the moved task in the new column
		m.positionCursorOnTask(msg.Task, msg.TargetSection)
		m.statusMessage = fmt.Sprintf("Moved to %s", sectionDisplayName(msg.TargetSection))
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
		// (completed/canceled tasks are filtered out of Kanban view)
		m.loadTasks()
		// Try to position cursor on the task if it's still visible
		for _, section := range KanbanColumnOrder {
			visibleNodes := m.visibleNodesForSection(section)
			for i, node := range visibleNodes {
				if node.Task.ID == msg.TaskID {
					// Found the task - position cursor on it
					for colIdx, sec := range KanbanColumnOrder {
						if sec == section {
							m.ActiveColumn = colIdx
							m.Columns[section].Cursor = i
							return m, nil
						}
					}
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
			section := msg.Task.Section
			// Find the column for this section
			for colIdx, sec := range KanbanColumnOrder {
				if sec == section {
					m.ActiveColumn = colIdx
					break
				}
			}
			m.positionCursorOnTask(msg.Task, section)
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
			section := msg.Task.Section
			// Find the column for this section
			for colIdx, sec := range KanbanColumnOrder {
				if sec == section {
					m.ActiveColumn = colIdx
					break
				}
			}
			m.positionCursorOnTask(msg.Task, section)
		}
		return m, nil
	}

	return m, nil
}

// handleKeyMsg processes keyboard input for the Kanban view.
func (m *KanbanViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle quick edit mode input first
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		return m.handleQuickEditInput(msg)
	}

	// Handle quick add mode input first
	if m.quickAddMode {
		return m.handleQuickAddInput(msg)
	}

	// Handle delete confirmation
	if m.confirmDelete {
		switch key {
		case "y", "Y":
			m.confirmDelete = false
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkDelete()
			}
			return m, m.executeDelete()
		case "n", "N", "esc":
			m.confirmDelete = false
			m.deleteTarget = nil
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	// Handle archive confirmation
	if m.confirmArchive {
		switch key {
		case "y", "Y":
			m.confirmArchive = false
			return m, m.executeArchiveToggle()
		case "n", "N", "esc":
			m.confirmArchive = false
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	// Handle completed view mode input
	if m.completedViewMode {
		if handled, _ := m.handleCompletedViewKey(key); handled {
			// Check for pending prefix from completed view
			if m.pendingPrefix != "" {
				return m, nil
			}
			return m, nil
		}
	}

	// Handle search mode input first
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	// Handle pending prefix keys first (two-key sequences like ",s")
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// TAB toggles focus between kanban and details panes
	if key == "tab" {
		m.detailsPaneFocused = !m.detailsPaneFocused
		if m.detailsPaneFocused {
			m.detailsScroll = 0 // Reset scroll when focusing details
		}
		return m, nil
	}

	// When details pane is focused, handle its specific keys
	if m.detailsPaneFocused {
		return m.handleDetailsPaneKey(key)
	}

	// Kanban pane key handling
	switch key {
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewKanban,
				ViewName:   "Kanban",
				Items:      m.GetHelpItems(),
			}
		}

	case "esc", "ctrl+k":
		// Exit multi-select mode first
		if m.multiSelectMode {
			m.multiSelectMode = false
			m.selectedTasks = make(map[string]bool)
			m.statusMessage = ""
			return m, nil
		}
		// Exit completed view mode
		if m.completedViewMode {
			m.completedViewMode = false
			m.statusMessage = ""
			return m, nil
		}
		// Exit quick edit mode
		if m.quickEdit != nil && m.quickEdit.IsActive() {
			m.quickEdit.Exit()
			m.statusMessage = ""
			return m, nil
		}
		// Exit focus mode if active
		if m.FocusedTask != nil {
			m.exitFocusMode()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "q":
		// Exit focus mode first if active
		if m.FocusedTask != nil {
			m.exitFocusMode()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	// Vertical navigation within current column
	case "up", "k":
		m.moveCursorUp()
		return m, nil

	case "down", "j":
		m.moveCursorDown()
		return m, nil

	// Horizontal navigation between columns
	case "h":
		m.moveColumnLeft()
		return m, nil

	case "l":
		m.moveColumnRight()
		return m, nil

	// Indent/outdent (hierarchy adjustment)
	case "left":
		return m, m.outdentTask()

	case "right":
		return m, m.indentTask()

	// Jump to last task in column
	case "G":
		// Go to last visible node in current column
		visibleCount := m.currentVisibleNodeCount()
		if visibleCount > 0 {
			cs := m.CurrentColumnState()
			cs.Cursor = visibleCount - 1
		}
		return m, nil

	// Task movement between columns (use {/} to free H/L for type cycling)
	case "{":
		// Move selected task to the left column
		return m, m.moveTaskLeft()

	case "}":
		// Move selected task to the right column
		return m, m.moveTaskRight()

	// Task reordering within column
	case "J":
		// Swap selected task with task below
		return m, m.swapTaskDown()

	case "K":
		// Swap selected task with task above
		return m, m.swapTaskUp()

	// Prefix key activation for Vim-style two-key sequences
	case ",":
		m.pendingPrefix = ","
		return m, nil

	case "!":
		m.pendingPrefix = "!"
		return m, nil

	case "d":
		m.pendingPrefix = "d"
		return m, nil

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "c":
		task := m.getSelectedTask()
		if task != nil {
			m.pendingPrefix = "c"
		}
		return m, nil

	case "z":
		// Fold prefix for expand/collapse operations
		m.pendingPrefix = "z"
		return m, nil

	// Sorting keys (1-4 for sort field, 0 to reset)
	case "1":
		m.cycleSortField(SortByPriority)
		return m, nil

	case "2":
		m.cycleSortField(SortByDueDate)
		return m, nil

	case "3":
		m.cycleSortField(SortByCreatedAt)
		return m, nil

	case "4":
		m.cycleSortField(SortByStatus)
		return m, nil

	case "0":
		m.resetSort()
		return m, nil

	// Multi-select mode
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	case " ":
		// Space: toggle selection in multi-select mode
		if m.multiSelectMode {
			m.toggleTaskSelection()
		}
		return m, nil

	case "a":
		// Select all in column (only in multi-select mode)
		if m.multiSelectMode {
			m.selectAllInColumn()
		}
		return m, nil

	// Open task detail view via glow
	case "enter":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return m, nil

	// Edit task in configured editor
	case "e":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   tasks,
					Project: project,
				}
			}
		}
		return m, m.handleEditTask()

	// Quick edit (inline with modifiers)
	case "E":
		return m, m.handleQuickEditKey()

	// Rename (inline title only)
	case "r":
		return m, m.handleRenameKey()

	// Filter keybindings
	case "F":
		// Toggle filter bar visibility
		if m.filterBar != nil {
			m.filterBar.Toggle()
		}
		return m, nil

	case "ctrl+f":
		// Show active filters overlay (same as project tree)
		return m, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m.projectTree}
		}

	case "f2":
		// Toggle high priority filter
		m.toggleKanbanPriorityFilter()
		m.loadTasks()
		return m, nil

	case "f3":
		// Cycle type filter
		m.cycleKanbanTypeFilter()
		m.loadTasks()
		return m, nil

	case "f4":
		// Cycle status filter
		m.cycleKanbanStatusFilter()
		m.loadTasks()
		return m, nil

	case "f5":
		// Tag filter selector
		return m, func() tea.Msg {
			return ShowKanbanTagFilterSelectorMsg{}
		}

	case "f1":
		// Toggle completed view
		m.toggleCompletedView()
		return m, nil

	case "f7":
		// Cycle due date filter
		m.cycleKanbanDueDateFilter()
		m.loadTasks()
		return m, nil

	case "f8":
		// Toggle actionable filter
		m.toggleActionableFilter()
		return m, nil

	// Tag filter selector (or bulk tag assignment in multi-select)
	case "T":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		return m, func() tea.Msg {
			return ShowKanbanTagFilterSelectorMsg{}
		}

	// Quick tag assignment
	case "ctrl+g":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			project := m.Project
			tags := append([]string{}, task.Tags...)
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        task,
					Project:     project,
					CurrentTags: tags,
				}
			}
		}
		return m, nil

	case "/":
		// Activate search mode
		m.searchMode = true
		m.searchInput.SetValue(m.filter.Search)
		m.searchInput.Focus()
		return m, textinput.Blink

	case "n":
		// Add new task with selected task as parent
		var parentTask *models.Task
		if m.FocusedTask != nil {
			parentTask = m.FocusedTask
		} else {
			parentTask = m.getSelectedTask()
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask}
		}

	case "i":
		// Open modal quick capture view
		return m, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.Project, AllProjects: m.config.Projects}
		}

	case "D":
		// Initiate delete confirmation
		if m.multiSelectMode && m.selectedCount() > 0 {
			m.confirmDelete = true
			m.statusMessage = fmt.Sprintf("Delete %d selected tasks? Press 'y' to confirm, 'n' to cancel", m.selectedCount())
			return m, nil
		}
		task := m.getSelectedTask()
		if task != nil {
			m.confirmDelete = true
			m.deleteTarget = task
			m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", task.ShortID)
		}
		return m, nil

	case "y":
		// Copy task title to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksTitleMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return m, nil

	case "Y":
		// Copy task description to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksDetailsMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return m, nil

	case "ctrl+y":
		// Copy task title and description to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksTitleAndDescMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: task}
			}
		}
		return m, nil

	// Comments view
	case "C":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskCommentsMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Attachments view (or select all in multi-select mode)
	case "A":
		if m.multiSelectMode {
			m.selectAllAcrossColumns()
			return m, nil
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskAttachmentsMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Dependency viewer (read-only)
	case "R":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowDependencyViewerMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Dependency manager
	case "ctrl+b":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ManageDependenciesMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Section picker
	case "ctrl+n":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Tag manager
	case "ctrl+t":
		return m, func() tea.Msg {
			return ShowTagManagerMsg{ProjectTree: nil}
		}

	// Section manager
	case "ctrl+s":
		return m, func() tea.Msg {
			return ShowSectionManagerMsg{Project: m.Project}
		}

	// Edit last modified task
	case "ctrl+l":
		recentTasks := storage.GetRecentTasks(m.Project.Tasks, 1)
		if len(recentTasks) > 0 {
			recentTask := recentTasks[0]
			// Find the task and navigate to it
			for _, section := range KanbanColumnOrder {
				visibleNodes := m.visibleNodesForSection(section)
				for i, node := range visibleNodes {
					if node.Task.ID == recentTask.ID {
						// Navigate to column and position
						for colIdx, sec := range KanbanColumnOrder {
							if sec == section {
								m.ActiveColumn = colIdx
								m.Columns[section].Cursor = i
								m.statusMessage = "Jumped to last modified"
								return m, m.handleEditTask()
							}
						}
					}
				}
			}
			m.statusMessage = "Last modified task not visible"
		} else {
			m.statusMessage = "No recently modified task"
		}
		return m, nil

	// Archive/unarchive project (with confirmation)
	case "ctrl+a":
		m.confirmArchive = true
		if m.Project.Archived {
			m.statusMessage = "Unarchive project? Press 'y' to confirm, 'n' to cancel"
		} else {
			m.statusMessage = "Archive project? Press 'y' to confirm, 'n' to cancel"
		}
		return m, nil

	// External editor for task content
	case "ctrl+o":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Move task to another project
	case "M":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: m.Project}
			}
		}
		return m, nil

	// Toggle pinned
	case "ctrl+p":
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, task := range m.getSelectedTasksList() {
				task.TogglePinned()
			}
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			m.statusMessage = fmt.Sprintf("Toggled pin on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		task := m.getSelectedTask()
		if task != nil {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.TogglePinned()
			m.statusMessage = fmt.Sprintf("Pinned: %v", task.Pinned)
			return m, m.saveWithStatusChange(prevStates, task)
		}
		return m, nil

	// Toggle active flag
	case "*":
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, task := range m.getSelectedTasksList() {
				task.ToggleActive()
			}
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			m.statusMessage = fmt.Sprintf("Toggled active on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		task := m.getSelectedTask()
		if task != nil {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.ToggleActive()
			m.statusMessage = fmt.Sprintf("Active: %v", task.Active)
			return m, m.saveWithStatusChange(prevStates, task)
		}
		return m, nil

	// Refresh from disk
	case "ctrl+r":
		_ = storage.LoadProject(m.Project)
		m.loadTasks()
		m.statusMessage = "Refreshed from disk"
		return m, nil

	// Tab switching
	case "[":
		if m.activeTab > 0 {
			m.applyTabPreset(m.activeTab - 1)
		}
		return m, nil

	case "]":
		if m.activeTab < len(kanbanTabNames)-1 {
			m.applyTabPreset(m.activeTab + 1)
		}
		return m, nil

	// Blockers view
	case "b":
		return m, func() tea.Msg {
			return ShowBlockersMsg{Project: m.Project}
		}

	// Priority view
	case "p":
		return m, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.Project}
		}

	// Inbox processor
	case "w":
		return m, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.Project}
		}

	// Focus shortcut (alias for zv)
	case "f":
		m.focusSubtree()
		return m, nil

	// Bulk capture
	case "I":
		return m, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.Project}
		}

	// Task jumper
	case "\\":
		return m, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.Project}
		}

	// Project cycling
	case "`":
		return m, func() tea.Msg {
			return CycleProjectMsg{Forward: true}
		}

	case "~":
		return m, func() tea.Msg {
			return CycleProjectMsg{Forward: false}
		}

	case "@":
		return m, func() tea.Msg {
			return ShowProjectCycleManagerMsg{}
		}

	// Type management: promote/demote
	case "<":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanPromote(); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.Promote()
				m.statusMessage = fmt.Sprintf("Promoted to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	case ">":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanDemote(); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.Demote()
				m.statusMessage = fmt.Sprintf("Demoted to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	// Type management: cycle leaf types (Task/Bug/Idea)
	case "H":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanCycleLeafType(true); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.CycleLeafType(true)
				m.statusMessage = fmt.Sprintf("Changed to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	case "L":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanCycleLeafType(false); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.CycleLeafType(false)
				m.statusMessage = fmt.Sprintf("Changed to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	// Undo
	case "u":
		return m, m.performUndo()

	// Undo history
	case "ctrl+z":
		return m, func() tea.Msg {
			return ShowUndoHistoryMsg{Project: m.Project}
		}
	}

	return m, nil
}

// handleSearchInput processes keyboard input while in search mode.
func (m *KanbanViewModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel search
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil

	case "enter":
		// Confirm search
		m.searchMode = false
		m.searchInput.Blur()
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, nil

	default:
		// Update search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Live filtering as user types
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, cmd
	}
}

// handleDetailsPaneKey handles keys when the details pane has focus.
func (m *KanbanViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return focus to kanban pane
		m.detailsPaneFocused = false
		return m, nil

	case "q":
		// Close the kanban view entirely
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "enter":
		// Open task detail view via glow
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return m, nil

	case "j", "down":
		// Scroll description down
		task := m.getSelectedTask()
		if task != nil && task.Description != "" {
			descLines := strings.Split(task.Description, "\n")
			maxScroll := len(descLines) - 3 // Keep at least 3 lines visible
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.detailsScroll < maxScroll {
				m.detailsScroll++
			}
		}
		return m, nil

	case "k", "up":
		// Scroll description up
		if m.detailsScroll > 0 {
			m.detailsScroll--
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewKanban,
				ViewName:   "Kanban",
				Items:      m.GetHelpItems(),
			}
		}
	}

	// Other keys are no-op when details pane is focused
	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
// It clears the pending prefix and processes the combined command.
func (m *KanbanViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		// Status operations (support bulk in multi-select mode)
		switch key {
		case "s":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusInProgress)
			}
			return m, m.handleStatusStart()
		case "d":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusDone)
			}
			return m, m.handleStatusDone()
		case "b":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusBlocked)
			}
			return m, m.handleStatusBlocked()
		case "o":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusTodo)
			}
			return m, m.handleStatusOpen()
		case "x":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusCanceled)
			}
			return m, m.handleStatusCancelled()
		}

	case "!":
		// Priority quick actions
		switch key {
		case "1", "2", "3", "4", "5":
			priority := int(key[0] - '0')
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkPriorityChange(priority)
			}
			task := m.getSelectedTask()
			if task != nil {
				prevStates := captureTaskStates(m.Project.Tasks)
				task.Priority = priority
				task.UpdatedAt = time.Now()
				m.statusMessage = fmt.Sprintf("Priority set to %d", priority)
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}

	case "d":
		// Due date quick actions
		now := time.Now()
		var dueDate *time.Time
		var desc string

		switch key {
		case "t":
			today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
			dueDate = &today
			desc = "today"
		case "m":
			tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 0, now.Location())
			dueDate = &tomorrow
			desc = "tomorrow"
		case "w":
			nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 0, now.Location())
			dueDate = &nextWeek
			desc = "+7 days"
		case "n":
			nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 23, 59, 59, 0, now.Location())
			dueDate = &nextMonth
			desc = "+30 days"
		case "0":
			dueDate = nil
			desc = "cleared"
		case "d":
			// Open full date picker (single task only)
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, nil
			}
			task := m.getSelectedTask()
			if task == nil {
				return m, nil
			}
			project := m.Project
			return m, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			}
		default:
			return m, nil
		}

		if m.multiSelectMode && m.selectedCount() > 0 {
			return m, m.handleBulkDueDateChange(dueDate, desc)
		}
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}
		prevStates := captureTaskStates(m.Project.Tasks)
		task.DueDate = dueDate
		task.UpdatedAt = time.Now()
		m.statusMessage = fmt.Sprintf("Due date %s", desc)
		return m, m.saveWithStatusChange(prevStates, task)

	case "g":
		// Go/section navigation and send operations
		switch key {
		case "g":
			// Go to first task in current column (vim-style gg)
			if m.completedViewMode {
				m.completedCursor = 0
				return m, nil
			}
			cs := m.CurrentColumnState()
			cs.Cursor = 0
			return m, nil
		case "c":
			// Toggle completed view
			m.toggleCompletedView()
			return m, nil
		case "a":
			// Go to Active column
			m.ActiveColumn = 0
			return m, nil
		case "n":
			// Go to Next column
			m.ActiveColumn = 1
			return m, nil
		case "i":
			// Go to Inbox column
			m.ActiveColumn = 2
			return m, nil
		case "w":
			// Go to Waiting column
			m.ActiveColumn = 3
			return m, nil
		case "s":
			// Go to Someday column
			m.ActiveColumn = 4
			return m, nil
		case "A":
			// Send task(s) to Active section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionActive)
			}
			return m, m.sendTaskToSection(KanbanSectionActive)
		case "N":
			// Send task(s) to Next section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionNext)
			}
			return m, m.sendTaskToSection(KanbanSectionNext)
		case "I":
			// Send task(s) to Inbox section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionInbox)
			}
			return m, m.sendTaskToSection(KanbanSectionInbox)
		case "W":
			// Send task(s) to Waiting section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionWaiting)
			}
			return m, m.sendTaskToSection(KanbanSectionWaiting)
		case "S":
			// Send task(s) to Someday section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionSomeday)
			}
			return m, m.sendTaskToSection(KanbanSectionSomeday)
		case "X":
			// Deselect all in multi-select mode
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		case "h":
			// Go to task activity history
			task := m.getSelectedTask()
			if task != nil {
				project := m.Project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "c":
		// Create linked task operations
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}
		project := m.Project

		// Navigate to linked tasks
		if key == "n" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
			}
		}

		// Unlink linked task
		if key == "x" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project, ForUnlink: true}
			}
		}

		// Cross-project linking
		if key == "p" {
			return m, func() tea.Msg {
				return ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: project}
			}
		}

		var linkType LinkType
		switch key {
		case "b":
			linkType = LinkTypeBlocker
		case "d":
			linkType = LinkTypeDependent
		case "s":
			linkType = LinkTypeSubtask
		case "f":
			linkType = LinkTypeFollowUp
		default:
			return m, nil
		}
		return m, func() tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: linkType}
		}

	case "z":
		// Fold/expand operations
		switch key {
		case "a":
			// Toggle expand on current node
			m.toggleCurrentExpand()
			return m, nil
		case "o":
			// Expand current node
			m.expandCurrent()
			return m, nil
		case "c":
			// Collapse current node
			m.collapseCurrent()
			return m, nil
		case "R":
			// Expand all in column
			m.expandAllInColumn()
			return m, nil
		case "M":
			// Collapse all in column
			m.collapseAllInColumn()
			return m, nil
		case "v":
			// Focus subtree (zoom into selected task)
			m.focusSubtree()
			return m, nil
		}
	}

	return m, nil
}

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

// moveColumnLeft moves focus to the previous non-empty column on the left.
// Empty columns are skipped. If no non-empty column exists to the left,
// focus remains on the current column.
func (m *KanbanViewModel) moveColumnLeft() {
	m.ActiveColumn = m.findNextNonEmptyColumn(-1)
}

// moveColumnRight moves focus to the next non-empty column on the right.
// Empty columns are skipped. If no non-empty column exists to the right,
// focus remains on the current column.
func (m *KanbanViewModel) moveColumnRight() {
	m.ActiveColumn = m.findNextNonEmptyColumn(+1)
}

// View implements the tea.Model interface. Renders a split view with:
// - Filter bar (when enabled): filter badges and column counts
// - Top half: kanban columns with section-colored headers
// - Horizontal divider
// - Bottom half: task details panel for the selected task
// When in completed view mode, renders a full-width list of completed tasks.
func (m *KanbanViewModel) View() string {
	if m.Project == nil {
		return MutedStyle.Render("No project loaded")
	}

	// Render completed view if in completed view mode
	if m.completedViewMode {
		return m.renderCompletedView()
	}

	var viewParts []string

	// Render tab bar at the top
	viewParts = append(viewParts, m.renderKanbanTabBar())

	// Render filter bar if enabled
	if m.filterBar != nil && m.filterBar.IsEnabled() {
		filterBar := m.renderKanbanFilterBar()
		if filterBar != "" {
			viewParts = append(viewParts, strings.TrimSuffix(filterBar, "\n"))
		}
	}

	// Render search input if in search mode
	if m.searchMode {
		searchLine := "Search: " + m.searchInput.View()
		viewParts = append(viewParts, searchLine)
	}

	var columns []string
	gap := " " // Single space gap between columns

	for i, section := range KanbanColumnOrder {
		columnContent := m.renderColumn(section, i == m.ActiveColumn)
		columns = append(columns, columnContent)
	}

	// Join columns horizontally with gaps
	var parts []string
	for i, col := range columns {
		parts = append(parts, col)
		if i < len(columns)-1 {
			parts = append(parts, gap)
		}
	}

	kanbanPane := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	// Check if all columns are empty (no tasks match filters)
	totalTasks := 0
	for _, section := range KanbanColumnOrder {
		totalTasks += len(m.Tasks[section])
	}
	if totalTasks == 0 && m.HasActiveFilters() {
		// Show message when filters result in no tasks
		emptyMsg := MutedStyle.Render("No tasks match current filters")
		kanbanPane = lipgloss.Place(m.Width(), m.kanbanHeight, lipgloss.Center, lipgloss.Center, emptyMsg)
	}

	viewParts = append(viewParts, kanbanPane)

	// Render the horizontal divider
	dividerStyle := MutedStyle
	if m.detailsPaneFocused {
		dividerStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	}
	divider := dividerStyle.Render(strings.Repeat("─", m.Width()))
	viewParts = append(viewParts, divider)

	// Render the details panel for the selected task
	tabBarHeight := 2 // Tab bar is always shown (text + bottom border)
	filterBarHeight := 0
	if m.filterBar != nil && m.filterBar.IsEnabled() {
		filterBarHeight = 2
	}
	if m.searchMode {
		filterBarHeight++
	}
	quickAddHeight := 0
	if m.quickAddMode {
		quickAddHeight = m.getQuickAddHeight()
	}
	quickEditHeight := m.getQuickEditHeight()
	statusBarHeight := 0
	if m.statusMessage != "" {
		statusBarHeight = 1
	}
	detailsHeight := m.Height() - m.kanbanHeight - 1 - tabBarHeight - filterBarHeight - quickAddHeight - quickEditHeight - statusBarHeight // -1 for divider
	if detailsHeight < 3 {
		detailsHeight = 3
	}
	selectedTask := m.getSelectedTask()
	detailsPane := m.renderDetailsPanel(selectedTask, m.Width(), detailsHeight)
	viewParts = append(viewParts, detailsPane)

	// Render quick add input if in quick add mode
	if m.quickAddMode {
		quickAddBar := m.renderQuickAddInput()
		viewParts = append(viewParts, quickAddBar)
	}

	// Render quick edit bar if in quick edit mode
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		quickEditBar := m.renderQuickEditBar()
		viewParts = append(viewParts, quickEditBar)
	}

	// Render status message if set
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		if m.confirmDelete || m.confirmArchive {
			statusStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		}
		viewParts = append(viewParts, statusStyle.Render(m.statusMessage))
	}

	// Render prefix popup if a prefix key is pending
	if popup := m.renderPrefixPopup(); popup != "" {
		viewParts = append(viewParts, popup)
	}

	// Join vertically: filterBar + kanban + divider + details + quickAdd + status + popup
	return lipgloss.JoinVertical(lipgloss.Left, viewParts...)
}

// renderKanbanTabBar renders the tab bar at the top of the kanban view.
func (m *KanbanViewModel) renderKanbanTabBar() string {
	availableWidth := m.Width()

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(ColorPrimary)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		BorderBottom(true).
		BorderStyle(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorBorder)

	var tabs []string
	for i, name := range kanbanTabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	tabWidth := lipgloss.Width(row)
	remaining := availableWidth - tabWidth
	if remaining > 0 {
		filler := lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder).
			Width(remaining).
			Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
	}

	return row
}

// renderColumn renders a single Kanban column with header and task list.
// Applies viewport scrolling when tasks exceed the available height.
func (m *KanbanViewModel) renderColumn(section string, isActive bool) string {
	var b strings.Builder

	// Render header with section color
	header := m.renderColumnHeader(section, isActive)
	b.WriteString(header)
	b.WriteString("\n")

	// Render visible nodes in this column
	visibleNodes := m.visibleNodesForSection(section)
	cs := m.Columns[section]

	// Calculate viewport height for tasks (kanbanHeight minus header line)
	viewportHeight := m.kanbanHeight - 1
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Always validate with reduced height to ensure cursor remains visible when
	// the "↑ N more" indicator appears. The indicator takes 1 line, so we
	// conservatively assume it will be shown during validation.
	reducedHeight := viewportHeight - 1
	if reducedHeight < 1 {
		reducedHeight = 1
	}
	cs.ValidateScroll(len(visibleNodes), reducedHeight)

	// Determine if indicator is actually needed after validation
	hasUpIndicator := cs.ScrollOffset > 0
	effectiveHeight := viewportHeight
	if hasUpIndicator {
		effectiveHeight = reducedHeight
	}

	if len(visibleNodes) == 0 {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		// Calculate visible range based on scroll offset
		startIdx := cs.ScrollOffset
		endIdx := startIdx + viewportHeight
		if endIdx > len(visibleNodes) {
			endIdx = len(visibleNodes)
		}

		// Show "↑ N more" indicator if tasks are hidden above
		if hasUpIndicator {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more", startIdx)))
			b.WriteString("\n")
			// Adjust visible range since we used a line for the indicator
			endIdx = startIdx + effectiveHeight
			if endIdx > len(visibleNodes) {
				endIdx = len(visibleNodes)
			}
		}

		// Render visible nodes
		for i := startIdx; i < endIdx; i++ {
			node := visibleNodes[i]
			isSelected := i == cs.Cursor && isActive && !m.detailsPaneFocused
			line := m.renderTaskRow(node, isSelected)
			b.WriteString(line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		// Show "↓ N more" indicator if tasks are hidden below
		if endIdx < len(visibleNodes) {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↓ %d more", len(visibleNodes)-endIdx)))
		}
	}

	// Pad column to exact height for consistent layout
	content := b.String()
	lines := strings.Split(content, "\n")
	for len(lines) < m.kanbanHeight {
		lines = append(lines, "")
	}
	if len(lines) > m.kanbanHeight {
		lines = lines[:m.kanbanHeight]
	}

	// Apply column width styling
	columnStyle := lipgloss.NewStyle().Width(m.ColumnWidth)
	return columnStyle.Render(strings.Join(lines, "\n"))
}

// renderColumnHeader renders the column header with appropriate section color.
// Uses the same ViewMode styles as the project tree view mode indicator for consistency.
func (m *KanbanViewModel) renderColumnHeader(section string, isActive bool) string {
	displayName := sectionDisplayName(section)
	visibleCount := m.visibleNodeCount(section)
	totalCount := len(m.Tasks[section])

	// Show visible/total if different (collapsed nodes)
	var headerText string
	if visibleCount != totalCount {
		headerText = fmt.Sprintf("%s (%d/%d)", displayName, visibleCount, totalCount)
	} else {
		headerText = fmt.Sprintf("%s (%d)", displayName, totalCount)
	}

	// Append sort indicator if this column has a non-default sort
	sortField := m.getSortFieldForColumn(section)
	if sortField != SortByDefault {
		direction := "↑"
		if isActive && !m.sortAscending {
			direction = "↓"
		}
		headerText += " " + direction + sortField.ShortString()
	}

	// Use the same ViewMode style as the view mode indicator
	style := GetViewModeStyleForSection(section)

	// Add indicator if this column is active
	if isActive {
		headerText = "▸ " + headerText
	}

	return style.Render(headerText)
}

// renderTaskRow renders a single task row with hierarchy indicators.
// Shows indentation, expand/collapse icons, and children-elsewhere badges.
// In multi-select mode, shows [x] or [ ] checkbox prefix.
func (m *KanbanViewModel) renderTaskRow(node *KanbanNode, isSelected bool) string {
	task := node.Task
	statusIcon := task.Status.Icon()
	title := task.Title

	// Build selection checkbox for multi-select mode
	var checkbox string
	if m.multiSelectMode {
		if m.isTaskSelected(task.ID) {
			checkbox = SelectionStyle.Render("[x]") + " "
		} else {
			checkbox = MutedStyle.Render("[ ]") + " "
		}
	}

	// Build indent prefix: "└" for nested items
	var indent string
	if node.Level > 0 {
		indent = strings.Repeat(" ", node.Level) + "└"
	}

	// Build expand/collapse icon
	var expandIcon string
	if node.HasChildrenHere {
		if node.Expanded {
			expandIcon = "▼"
		} else {
			expandIcon = "▶"
		}
	} else {
		expandIcon = " "
	}

	// Build children-elsewhere badge
	var badge string
	if node.ChildrenElsewhere > 0 {
		badge = MutedStyle.Render(fmt.Sprintf("+%d", node.ChildrenElsewhere))
	}

	// Calculate available space for title
	// Layout: cursor (3) + checkbox (4 if multi-select) + indent + expand (1) + space (1) + status (1) + space (1) + title + badge
	checkboxLen := 0
	if m.multiSelectMode {
		checkboxLen = 4 // "[x] " or "[ ] "
	}
	prefixLen := 3 + checkboxLen + len(indent) + 1 + 1 + 1 + 1
	badgeLen := 0
	if badge != "" {
		badgeLen = len(fmt.Sprintf("+%d", node.ChildrenElsewhere)) + 1 // +1 for space before badge
	}
	maxTitleLen := m.ColumnWidth - prefixLen - badgeLen
	if maxTitleLen < 5 {
		maxTitleLen = 5
	}
	title = truncateWithEllipsis(title, maxTitleLen)

	// Apply active/pinned title styling
	if isSelected {
		if task.Active {
			title = BoldStyle.Foreground(lipgloss.Color("226")).Render(title)
		} else if task.Pinned {
			title = BoldStyle.Foreground(ColorSuccess).Render(title)
		}
	} else if task.Active {
		title = ActiveStyle.Render(title)
	} else if task.Pinned {
		title = lipgloss.NewStyle().Foreground(ColorSuccess).Render(title)
	}

	statusStyle := GetStatusStyle(string(task.Status))
	content := checkbox + indent + expandIcon + " " + statusStyle.Render(statusIcon) + " " + title
	if badge != "" {
		content += " " + badge
	}

	// Build cursor prefix with active marker (matches RenderSelectableRow)
	activeMarker := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Render("*")
	if isSelected && task.Active {
		return SelectionCursorStyle.Render(">") + activeMarker + " " + content
	} else if isSelected {
		return SelectionCursorStyle.Render(">") + "  " + content
	} else if task.Active {
		return " " + activeMarker + " " + content
	}
	return "   " + content
}

// truncateWithEllipsis truncates a string to maxLen characters, adding "..."
// suffix if truncation occurs. Returns original string if it fits within maxLen.
func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	// Need at least 4 chars to show something meaningful with ellipsis (1 char + "...")
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// sectionDisplayName returns the display name for a section.
// The Active section (empty string) displays as "Active".
func sectionDisplayName(section string) string {
	if section == "" {
		return "Active"
	}
	return section
}

// renderDetailsPanel renders the task details panel for the bottom half of the split view.
// Displays task info in cards: Task | Info+Due stacked | Description (double width)
func (m *KanbanViewModel) renderDetailsPanel(task *models.Task, width, height int) string {
	if task == nil {
		// Center "No task selected" message
		emptyMsg := MutedStyle.Render("No task selected")
		lines := make([]string, height)
		midLine := height / 2
		for i := range lines {
			if i == midLine {
				// Center horizontally
				padding := (width - lipgloss.Width(emptyMsg)) / 2
				if padding < 0 {
					padding = 0
				}
				lines[i] = strings.Repeat(" ", padding) + emptyMsg
			} else {
				lines[i] = ""
			}
		}
		return strings.Join(lines, "\n")
	}

	// Get section color for accent
	sectionColor := m.Project.GetSectionColor(task.Section)

	// Calculate column widths: 3 columns, but Description takes 2 columns worth
	col1Width := width / 4                     // Task card
	col2Width := width / 4                     // Info + Due stacked
	col3Width := width - col1Width - col2Width // Description (remaining ~half)

	// Column 1: Task card
	taskCard := m.renderDetailsTaskCard(task, col1Width, sectionColor)

	// Column 2: Info + Due stacked vertically
	infoCard := m.renderDetailsInfoCard(task, col2Width, sectionColor)
	var col2Content string
	if task.DueDate != nil {
		dueCard := m.renderDetailsDueCard(task, col2Width)
		col2Content = lipgloss.JoinVertical(lipgloss.Left, infoCard, dueCard)
	} else {
		col2Content = infoCard
	}

	// Column 3: Description (double width)
	var descCard string
	if task.Description != "" {
		descCard = m.renderDetailsDescriptionCard(task, col3Width, height)
	} else {
		descCard = renderSidebarCard("Description", MutedStyle.Render("(none)"), col3Width, false, "")
	}

	// Join columns horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, taskCard, col2Content, descCard)

	// Pad to exact height (minus 1 for help hint line which is part of app status bar)
	return m.padDetailsToHeight(content, width, height)
}

// renderDetailsTaskCard renders the main task info card for the details panel.
func (m *KanbanViewModel) renderDetailsTaskCard(task *models.Task, width int, sectionColor string) string {
	typeStyle := GetTypeStyle(string(task.Type))
	statusStyle := GetStatusStyle(string(task.Status))

	if m.Project.HasIncompleteDependencies(task) {
		statusStyle = BlockedByDepsStyle
	}

	title := SidebarHeaderStyle.Render(task.Title)
	typeLine := GetTypeBadge(string(task.Type), task.Type.Icon()) + " " + typeStyle.Render(task.Type.HeadingPrefix())
	statusLine := statusStyle.Render(task.Status.Icon() + " " + string(task.Status))
	priorityBadge := formatPriorityBadge(task.Priority)
	idLine := fmt.Sprintf("#%d %s", task.ShortID, priorityBadge)

	content := title + "\n" + typeLine + " " + statusLine + " " + idLine
	return renderSidebarCard("Task", content, width, m.detailsPaneFocused, sectionColor)
}

// renderDetailsInfoCard renders tags and section info for the details panel.
func (m *KanbanViewModel) renderDetailsInfoCard(task *models.Task, width int, sectionColor string) string {
	var lines []string

	// Tags line
	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagParts = append(tagParts, TagInlineStyle.Render("#"+tag))
		}
		lines = append(lines, strings.Join(tagParts, " "))
	}

	// Section line
	sectionName := task.Section
	if sectionName == "" {
		sectionName = "Active"
	}
	sectionStyle := GetSectionColorStyle(sectionColor)
	sectionLine := MutedStyle.Render("Section: ") + sectionStyle.Render(sectionName)
	lines = append(lines, sectionLine)

	return renderSidebarCard("Info", strings.Join(lines, "\n"), width, false, sectionColor)
}

// renderDetailsDueCard renders the due date card for the details panel.
func (m *KanbanViewModel) renderDetailsDueCard(task *models.Task, width int) string {
	dueAbsolute := task.DueDate.Format("2006-01-02")
	dueRelative := formatRelativeDueDateVerbose(task.DueDate)
	dueLine := fmt.Sprintf("%s (%s)", dueAbsolute, dueRelative)

	if isOverdue(task) && !task.Status.IsComplete() {
		dueLine = OverdueStyle.Render(dueLine)
	} else if isDueToday(task) {
		dueLine = DueTodayStyle.Render(dueLine)
	} else {
		dueLine = DueDateStyle.Render(dueLine)
	}

	return renderSidebarCard("Due", dueLine, width, false, "")
}

// renderDetailsDescriptionCard renders the scrollable description card.
func (m *KanbanViewModel) renderDetailsDescriptionCard(task *models.Task, width, availableHeight int) string {
	lines := strings.Split(task.Description, "\n")

	// Apply scroll offset
	if m.detailsScroll < len(lines) {
		lines = lines[m.detailsScroll:]
	} else {
		lines = []string{}
	}

	// Limit visible lines
	maxLines := 5
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, MutedStyle.Render("..."))
	}

	return renderSidebarCard("Description", strings.Join(lines, "\n"), width, false, "")
}

// padDetailsToHeight ensures the details panel fills the exact height.
func (m *KanbanViewModel) padDetailsToHeight(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Truncate if too tall
	if len(lines) > height {
		lines = lines[:height]
	}

	// Pad if too short
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// ShouldCaptureEsc implements ViewCapabilities. Returns true when in any
// internal mode that should capture Esc instead of allowing app-level
// navigation. Currently tracks pending status prefix mode, details pane focus,
// search mode, quick add mode, delete confirmation, note conversion confirmation,
// archive confirmation, focus mode, multi-select mode, quick edit mode, and completed view mode.
func (m *KanbanViewModel) ShouldCaptureEsc() bool {
	return m.pendingPrefix != "" || m.detailsPaneFocused || m.searchMode || m.quickAddMode || m.confirmDelete || m.confirmArchive || m.FocusedTask != nil || m.multiSelectMode || (m.quickEdit != nil && m.quickEdit.IsActive()) || m.completedViewMode
}

// renderPrefixPopup renders the which-key style popup for pending prefix keys.
func (m *KanbanViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
		"g": KanbanNavigationPrefixHints,
		"c": CreateLinkedPrefixHints,
		"z": KanbanFoldPrefixHints,
	}, m.Width())
}
