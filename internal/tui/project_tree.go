package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
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
	IsContextNode       bool   // True if visible only to show hierarchy context (parent in different section)
	HasFilteredChildren bool   // True if any children pass current filters (used for collapse arrow)
	MatchPositions      []int  // Match positions within title for search highlighting
	TreePrefix          string // Pre-computed connector prefix (├── , └── , etc.)
}

// ProjectTreeModel represents the project tree view.
type ProjectTreeModel struct {
	BaseSearchableView
	InteractiveViewState
	PrimaryViewBundle

	// Queue indicator support
	queueIDs map[string]bool

	project *models.Project
	nodes   []*TreeNode

	// View mode (Active, Inbox, or Backlog)
	viewMode ViewMode

	// Filter options
	filter FilterState

	// Sort options
	sort SortState // current sort field and direction

	// Task to delete when confirm action is "delete" or "delete-recurring"
	deleteTarget *models.Task

	// Multi-select state
	MultiSelectState

	// Components
	filterBar *FilterBarComponent
	sidebar   *SidebarComponent

	// Custom section tab (0 = "All", 1+ = custom section index)
	customSectionTab int

	// Focus mode state
	focusedTask *models.Task // When non-nil, only show this task and descendants

	// Score filter custom value input
	scoreFilterInput ScoreFilterInput

	// Undo context for the next save operation
	undo UndoContext

	// Active saved filter name (set when a saved filter is loaded, cleared on manual filter/sort changes)
	activeSavedFilter string

	// Active criteria filter (set when a criteria-mode SavedFilter is loaded)
	activeCriteriaFilter *config.SavedFilter

	// Column visibility config (loaded from project view state)
	columnConfig *ColumnConfig

	// Inline preview mode: show description/checklist below cursor task
	previewMode bool

	// colLayout is computed once before each render pass and used by
	// renderNodeWithWidth to align badge columns across visible rows.
	// Nil when column alignment is disabled (e.g., narrow terminals).
	colLayout *ColumnLayout
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
	if err := storage.LoadProject(m.project); err != nil {
		logging.Warn("failed to load project %s: %v", m.project.Name, err)
		m.buildTree()
		return func() tea.Msg { return StatusMsg(fmt.Sprintf("Failed to reload project: %v", err)) }
	}
	m.buildTree()
	return nil
}

// clearActiveSavedFilter clears the active saved filter indicator and any
// criteria filter. Called when the user manually changes filter, sort, or view mode.
func (m *ProjectTreeModel) clearActiveSavedFilter() {
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
}

// NewProjectTreeModel creates a new project tree model.
func NewProjectTreeModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectTreeModel {
	var treeToggles map[string]bool
	var treePriorityMode string
	var treeDefaultColumns map[string]bool
	if cfg != nil {
		if vs, ok := cfg.ProjectViewStates[project.ID]; ok {
			treeToggles = vs.TreeColumnToggles
			treePriorityMode = vs.TreePriorityMode
		}
		if treePriorityMode == "" {
			treePriorityMode = cfg.TreePriorityMode
		}
		treeDefaultColumns = cfg.TreeDefaultColumns
	}
	m := &ProjectTreeModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, manager),
		project:           project,
		filter:            NewFilterState(),
		sort:              NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		filterBar:         NewFilterBarComponent(),
		sidebar:           NewSidebarComponent(),
		columnConfig:      NewColumnConfig(treeToggles, treeDefaultColumns, DefaultTreeColumns, treePriorityMode),
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
	return m.DefaultHasTextInput() || m.quickEdit.IsActive() || m.scoreFilterInput.IsActive()
}

// escSteps defines the Esc dismissal cascade for ProjectTree:
// multi-select → focus mode → search query.
func (m *ProjectTreeModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.clearSelection()
		}},
		{Active: m.IsFocusMode, Clear: func() {
			m.focusedTask = nil
			m.buildTree()
			m.clampCursor()
		}},
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.updateVisibility()
		}},
	}
}

// ShouldCaptureEsc returns true when the tree needs esc/q for internal modes.
func (m *ProjectTreeModel) ShouldCaptureEsc() bool {
	return primaryViewShouldCaptureEsc(&m.InteractiveViewState, &m.BaseSearchableView, m.multiSelectMode, m.quickEdit.IsActive(), m.scoreFilterInput.IsActive()) ||
		AnyEscStepActive(m.escSteps()) || m.sidebar.IsFocused()
}

// IsFocusMode returns true if the view is focused on a subtree.
func (m *ProjectTreeModel) IsFocusMode() bool {
	return m.focusedTask != nil
}

// clampCursor ensures the cursor is within valid bounds.
func (m *ProjectTreeModel) clampCursor() {
	m.ClampCursorTo(len(m.visibleNodes()))
}

func (m *ProjectTreeModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectTreeModel) StatusBarLabel() string {
	if m.project != nil {
		status := fmt.Sprintf("Project: %s", m.project.Name) + FormatCyclePosition(m.config, m.project.ID)
		if m.sidebar.IsFocused() {
			status += " " + SidebarFocusBadgeStyle.Render("[SIDEBAR]")
		}
		return status
	}
	return "Project"
}
