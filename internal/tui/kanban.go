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

	// Columns maps section name to column state.
	// Keys: "" (Active), "Next", "Inbox", "Waiting", "Someday"
	Columns map[string]*ColumnState

	// Tasks maps section name to the list of non-completed tasks in that section.
	// This is populated during construction and should be refreshed when tasks change.
	Tasks map[string][]*models.Task

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
		filter: KanbanFilterState{
			Tags: make(map[string]bool),
		},
		filterBar:     NewKanbanFilterBarComponent(),
		searchInput:   searchInput,
		quickAddInput: quickAddInput,
	}

	// Initialize column state for each of the five sections
	for _, section := range KanbanColumnOrder {
		m.Columns[section] = &ColumnState{}
		m.Tasks[section] = []*models.Task{}
	}

	// Load tasks into columns by section, excluding completed tasks
	m.loadTasks()

	return m
}

// loadTasks populates the Tasks map by filtering project tasks into columns.
// Only non-completed tasks are included. Each task is placed in the column
// matching its Section field (empty string maps to Active column).
// Filters are applied to determine which tasks are visible.
func (m *KanbanViewModel) loadTasks() {
	// Clear existing tasks
	for _, section := range KanbanColumnOrder {
		m.Tasks[section] = []*models.Task{}
	}

	if m.Project == nil {
		return
	}

	// Filter and categorize tasks by section
	for _, task := range m.Project.Tasks {
		// Skip completed tasks (done or canceled)
		if task.Status.IsComplete() {
			continue
		}

		// Apply filters - skip tasks that don't pass
		if !m.passesFilters(task) {
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

	// Validate cursor positions for each column after loading
	for _, section := range KanbanColumnOrder {
		taskCount := len(m.Tasks[section])
		m.Columns[section].Validate(taskCount)
	}

	// Calculate filter counts for the filter bar
	m.calculateFilterCounts()
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

	// Account for filter bar height when enabled (2 lines)
	filterBarHeight := 0
	if m.filterBar != nil && m.filterBar.IsEnabled() {
		filterBarHeight = 2
	}

	// Compute split view heights: 50/50 split minus 1 for the divider line
	// kanbanHeight = (height - 1 - filterBarHeight) / 2, leaving rest for details pane
	availableHeight := height - 1 - filterBarHeight // -1 for divider
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
		cs.Validate(len(m.Tasks[msg.Section]))
		return m, nil

	case KanbanStatusChangedMsg:
		// Task status changed - reload tasks since task may have moved columns
		// (completed/cancelled tasks are filtered out of Kanban view)
		m.loadTasks()
		// Try to position cursor on the task if it's still visible
		for _, section := range KanbanColumnOrder {
			for i, task := range m.Tasks[section] {
				if task.ID == msg.TaskID {
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
		// Task not visible (likely completed/cancelled) - validate current cursor
		cs := m.CurrentColumnState()
		cs.Validate(len(m.CurrentColumnTasks()))
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
		cs.Validate(len(m.Tasks[msg.Section]))
		m.statusMessage = "Task deleted"
		return m, nil
	}

	return m, nil
}

// handleKeyMsg processes keyboard input for the Kanban view.
func (m *KanbanViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle quick add mode input first
	if m.quickAddMode {
		return m.handleQuickAddInput(msg)
	}

	// Handle delete confirmation
	if m.confirmDelete {
		switch key {
		case "y", "Y":
			m.confirmDelete = false
			return m, m.executeDelete()
		case "n", "N", "esc":
			m.confirmDelete = false
			m.deleteTarget = nil
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
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
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "q":
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
	case "left", "h":
		m.moveColumnLeft()
		return m, nil

	case "right", "l":
		m.moveColumnRight()
		return m, nil

	// Jump to last task in column
	case "G":
		// Go to last task in current column
		tasks := m.CurrentColumnTasks()
		if len(tasks) > 0 {
			cs := m.CurrentColumnState()
			cs.Cursor = len(tasks) - 1
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
		return m, m.handleEditTask()

	// Filter keybindings
	case "F":
		// Toggle filter bar visibility
		if m.filterBar != nil {
			m.filterBar.Toggle()
		}
		return m, nil

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

	case "f7":
		// Cycle due date filter
		m.cycleKanbanDueDateFilter()
		m.loadTasks()
		return m, nil

	case "/":
		// Activate search mode
		m.searchMode = true
		m.searchInput.SetValue(m.filter.Search)
		m.searchInput.Focus()
		return m, textinput.Blink

	case "a":
		// Activate quick add mode
		m.quickAddMode = true
		m.initQuickAddCaches()
		section := m.CurrentSection()
		// Pre-fill section based on current column
		// Active column (empty string) doesn't need a section prefix
		if section != "" {
			m.quickAddInput.SetValue("/" + strings.ToLower(section) + " ")
			m.quickAddInput.SetCursor(len(m.quickAddInput.Value()))
		} else {
			m.quickAddInput.SetValue("")
		}
		m.quickAddInput.Focus()
		m.updateQuickAddPreview()
		return m, textinput.Blink

	case "D":
		// Initiate delete confirmation
		task := m.getSelectedTask()
		if task != nil {
			m.confirmDelete = true
			m.deleteTarget = task
			m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", task.ShortID)
		}
		return m, nil

	case "y":
		// Copy task title to clipboard
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return m, nil

	case "Y":
		// Copy task description to clipboard
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return m, nil

	case "ctrl+y":
		// Copy task title and description to clipboard
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

	// Attachments view
	case "A":
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

	// View scratchpad
	case "[":
		return m, func() tea.Msg {
			return ShowScratchpadMsg{Project: m.Project}
		}

	// Edit scratchpad
	case "]":
		return m, func() tea.Msg {
			return EditScratchpadMsg{Project: m.Project}
		}

	// Blockers view
	case "b":
		return m, func() tea.Msg {
			return ShowBlockersMsg{Project: m.Project}
		}

	// Bulk capture
	case "I":
		return m, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.Project}
		}

	// Create note
	case "o":
		var parentTask *models.Task
		task := m.getSelectedTask()
		if task != nil {
			parentTask = task
		}
		return m, func() tea.Msg {
			return CreateNoteMsg{ParentTask: parentTask}
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
		// Status operations
		switch key {
		case "s":
			return m, m.handleStatusStart()
		case "d":
			return m, m.handleStatusDone()
		case "b":
			return m, m.handleStatusBlocked()
		case "o":
			return m, m.handleStatusOpen()
		case "x":
			return m, m.handleStatusCancelled()
		}

	case "!":
		// Priority quick actions
		switch key {
		case "1", "2", "3", "4", "5":
			priority := int(key[0] - '0')
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
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}

		now := time.Now()
		var dueDate *time.Time
		var desc string

		switch key {
		case "t":
			// Today
			today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
			dueDate = &today
			desc = "today"
		case "m":
			// Tomorrow
			tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 0, now.Location())
			dueDate = &tomorrow
			desc = "tomorrow"
		case "w":
			// Next week (+7 days)
			nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 0, now.Location())
			dueDate = &nextWeek
			desc = "+7 days"
		case "n":
			// Next month (+30 days)
			nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 23, 59, 59, 0, now.Location())
			dueDate = &nextMonth
			desc = "+30 days"
		case "0":
			// Clear due date
			dueDate = nil
			desc = "cleared"
		case "d":
			// Open full date picker
			project := m.Project
			return m, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			}
		default:
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
			cs := m.CurrentColumnState()
			cs.Cursor = 0
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
			// Send task to Active section
			return m, m.sendTaskToSection(KanbanSectionActive)
		case "N":
			// Send task to Next section
			return m, m.sendTaskToSection(KanbanSectionNext)
		case "I":
			// Send task to Inbox section
			return m, m.sendTaskToSection(KanbanSectionInbox)
		case "W":
			// Send task to Waiting section
			return m, m.sendTaskToSection(KanbanSectionWaiting)
		case "S":
			// Send task to Someday section
			return m, m.sendTaskToSection(KanbanSectionSomeday)
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
	}

	return m, nil
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
	tasks := m.CurrentColumnTasks()
	if cs.Cursor < len(tasks)-1 {
		cs.Cursor++
	}
}

// positionCursorOnTask positions the cursor on the specified task in the given section.
// If the task is not found, the cursor is positioned at the first task (or 0 if empty).
func (m *KanbanViewModel) positionCursorOnTask(task *models.Task, section string) {
	tasks := m.Tasks[section]
	cs := m.Columns[section]

	// Find the task in the column
	for i, t := range tasks {
		if t.ID == task.ID {
			cs.Cursor = i
			return
		}
	}

	// Task not found, validate cursor to be in bounds
	cs.Validate(len(tasks))
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
func (m *KanbanViewModel) View() string {
	if m.Project == nil {
		return MutedStyle.Render("No project loaded")
	}

	var viewParts []string

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
	statusBarHeight := 0
	if m.statusMessage != "" {
		statusBarHeight = 1
	}
	detailsHeight := m.Height() - m.kanbanHeight - 1 - filterBarHeight - quickAddHeight - statusBarHeight // -1 for divider
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

	// Render status message if set
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		if m.confirmDelete {
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

// renderColumn renders a single Kanban column with header and task list.
// Applies viewport scrolling when tasks exceed the available height.
func (m *KanbanViewModel) renderColumn(section string, isActive bool) string {
	var b strings.Builder

	// Render header with section color
	header := m.renderColumnHeader(section, isActive)
	b.WriteString(header)
	b.WriteString("\n")

	// Render tasks in this column
	tasks := m.Tasks[section]
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
	cs.ValidateScroll(len(tasks), reducedHeight)

	// Determine if indicator is actually needed after validation
	hasUpIndicator := cs.ScrollOffset > 0
	effectiveHeight := viewportHeight
	if hasUpIndicator {
		effectiveHeight = reducedHeight
	}

	if len(tasks) == 0 {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		// Calculate visible range based on scroll offset
		startIdx := cs.ScrollOffset
		endIdx := startIdx + viewportHeight
		if endIdx > len(tasks) {
			endIdx = len(tasks)
		}

		// Show "↑ N more" indicator if tasks are hidden above
		if hasUpIndicator {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↑ %d more", startIdx)))
			b.WriteString("\n")
			// Adjust visible range since we used a line for the indicator
			endIdx = startIdx + effectiveHeight
			if endIdx > len(tasks) {
				endIdx = len(tasks)
			}
		}

		// Render visible tasks
		for i := startIdx; i < endIdx; i++ {
			task := tasks[i]
			isSelected := i == cs.Cursor && isActive && !m.detailsPaneFocused
			line := m.renderTaskRow(task, isSelected)
			b.WriteString(line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		// Show "↓ N more" indicator if tasks are hidden below
		if endIdx < len(tasks) {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("↓ %d more", len(tasks)-endIdx)))
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
	taskCount := len(m.Tasks[section])
	headerText := fmt.Sprintf("%s (%d)", displayName, taskCount)

	// Use the same ViewMode style as the view mode indicator
	style := GetViewModeStyleForSection(section)

	// Add indicator if this column is active
	if isActive {
		headerText = "▸ " + headerText
	}

	return style.Render(headerText)
}

// renderTaskRow renders a single task row, with selection indicator if selected.
func (m *KanbanViewModel) renderTaskRow(task *models.Task, isSelected bool) string {
	// Build task display: status icon + truncated title
	statusIcon := task.Status.Icon()
	title := task.Title

	// Truncate title to fit in column (accounting for icon and spacing)
	// Layout: "  " or "> " (2 chars) + icon (1 char) + " " (1 char) + title
	maxTitleLen := m.ColumnWidth - 4
	title = truncateWithEllipsis(title, maxTitleLen)

	statusStyle := GetStatusStyle(string(task.Status))
	content := statusStyle.Render(statusIcon) + " " + title

	if isSelected {
		return SelectionCursorStyle.Render(">") + " " + content
	}
	return "  " + content
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
// search mode, quick add mode, and delete confirmation.
func (m *KanbanViewModel) ShouldCaptureEsc() bool {
	return m.pendingPrefix != "" || m.detailsPaneFocused || m.searchMode || m.quickAddMode || m.confirmDelete
}

// HasTextInput implements ViewCapabilities. Returns true when in search or quick add mode.
func (m *KanbanViewModel) HasTextInput() bool {
	return m.searchMode || m.quickAddMode
}

// renderPrefixPopup renders the which-key style popup for pending prefix keys.
func (m *KanbanViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
		"g": KanbanNavigationPrefixHints,
		"c": CreateLinkedPrefixHints,
	}, m.Width())
}
