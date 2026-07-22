package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// PriorityGroup represents a category of tasks by urgency.
type PriorityGroup int

const (
	PriorityGroupOverdue PriorityGroup = iota
	PriorityGroupToday
	PriorityGroupThisWeek
	PriorityGroupLater
	PriorityGroupBacklog
	PriorityGroupWaitingOn
)

// PriorityTask wraps a task with computed urgency metadata.
type PriorityTask struct {
	Task         *models.Task
	UrgencyScore int
	ParentTitle  string // Cached parent context
	RelativeDue  string // Formatted relative due date
}

// SuggestedTask represents the single most actionable task with reasoning.
type SuggestedTask struct {
	Task   *PriorityTask
	Score  int
	Reason string
}

// Priority view tab names and indices.
var priorityTabNames = []string{"All", "Urgent", "Upcoming", "Backlog", "Blocked", "By Score"}

const (
	priorityTabAll      = 0
	priorityTabUrgent   = 1
	priorityTabUpcoming = 2
	priorityTabBacklog  = 3
	priorityTabBlocked  = 4
	priorityTabByScore  = 5
)

// PriorityViewModel displays actionable tasks sorted by urgency.
type PriorityViewModel struct {
	BaseSearchableView
	InteractiveViewState
	PrimaryViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	project        *models.Project
	projectService *ProjectService

	// Grouped tasks by urgency
	overdue   []*PriorityTask
	today     []*PriorityTask
	thisWeek  []*PriorityTask
	later     []*PriorityTask
	backlog   []*PriorityTask // Tasks without due dates
	waitingOn []*PriorityTask // Blocked tasks

	// Score-ranked tasks (populated when scoring profile assigned)
	scoredTasks []*PriorityTask

	// Smart suggestion
	suggestion   *SuggestedTask  // The single most actionable task
	dismissedIDs map[string]bool // Task IDs dismissed this session

	// Tab filtering
	tabs TabState

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

	// Navigation
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Filter state
	FilterMixin

	// Multi-select
	MultiSelectState

	// Waiting On section state
	waitingOnCollapsed bool

	// Inline preview
	previewMode bool
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project, runtime *config.RuntimeState, mgr *config.Manager) *PriorityViewModel {
	m := &PriorityViewModel{
		PrimaryViewBundle:  NewPrimaryViewBundle(mgr, runtime),
		project:            project,
		projectService:     NewProjectService(),
		waitingOnCollapsed: true,
		dismissedIDs:       make(map[string]bool),
		FilterMixin:        FilterMixin{filter: newFilterState()},
		MultiSelectState:   MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:               TabState{Names: priorityTabNames},
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	// Uppercase-X deletion now routes through the shell-owned canonical
	// TaskDeleteRequestedMsg confirmation, so Priority keeps no view-local delete
	// confirm modal.
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.InitSearchInput("Search tasks...", 100, 30)
	m.loadActionableTasks()
	return m
}

func (m *PriorityViewModel) Init() tea.Cmd {
	return nil
}

func (m *PriorityViewModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *PriorityViewModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.resetViewScrollOffset()
}

func (m *PriorityViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(
		msg,
		m.SetSize,
		m.loadActionableTasks,
		m.clampCursor,
		func(s string) { m.setStatus(s) },
	); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeypress(msg)
	}

	return m, nil
}

// escSteps defines the Esc dismissal cascade: search → filters.
func (m *PriorityViewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.recalculateNavigation()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.loadActionableTasks()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when the priority view is in prefix mode, confirm dialog, search mode, or filters are active.
func (m *PriorityViewModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || AnyEscStepActive(m.escSteps())
}

// HasTextInput returns true when the search input is active.
func (m *PriorityViewModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}

func (m *PriorityViewModel) View() tea.View {
	var b strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks · %s", m.totalTasks, m.project.Name)
	b.WriteString(renderFullscreenTitle("Priority View", subtitle, m.AccentColor()))
	b.WriteString("\n")

	_, _, _, _, _, filteredWaiting := m.filteredGroupsWithSubTab()

	if m.totalTasks == 0 && len(filteredWaiting) == 0 {
		b.WriteString(m.renderPriorityTabBar())
		b.WriteString("\n\n")
		b.WriteString(renderFilterAwareEmptyState(
			m.searchQuery != "",
			EmptyStateNoTasks, "",
			"Try a different search term.",
		))
		b.WriteString("\n\n")
		if m.IsSearchMode() {
			b.WriteString(HelpStyle.Render("Search: ") + m.SearchInputView())
			b.WriteString("\n")
		}
		b.WriteString(HelpStyle.Render("shift+←/→: tabs" + m.activeTabHint()))
		return tea.NewView(b.String())
	}

	// Pre-render conditional sections to calculate their actual height
	suggestionSection := ""
	if m.tabs.Active != priorityTabByScore {
		suggestionSection = m.renderSuggestionSection()
	}
	tabHint := m.activeTabHint()
	filterIndicator := m.renderActiveFilterIndicator()

	// Calculate available height for task list
	contentHeight := fullscreenContentHeight(m.Height(), 2) // tab bar(2)

	// Account for sub-tab bar
	hasSubTabs := len(m.subTabNames) > 1
	if hasSubTabs {
		contentHeight -= 2 // sub-tab row + newline
	}

	// Account for filter indicator if present
	if filterIndicator != "" {
		contentHeight-- // filter badge + newline
	}

	// Account for search bar if active
	if m.IsSearchMode() {
		contentHeight-- // search input line
	}

	// Account for suggestion section if present
	if suggestionSection != "" {
		suggestionLines := lipgloss.Height(suggestionSection)
		contentHeight -= suggestionLines
	}

	// Reserve one row for the selected-task metadata strip whenever the cursor
	// is in bounds. Keeps layout stable as the selection moves.
	var stripTask *models.Task
	var stripProject *models.Project
	if pt := m.getTaskAtCursor(); pt != nil {
		stripTask = pt.Task
		stripProject = m.project
		contentHeight = max(contentHeight-1, 1)
	}

	// Render groups
	currentIndex := 0
	lines, cursorLineIdx := m.renderAllGroups(&currentIndex)

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Insert inline preview after cursor
	if m.previewMode && cursorLineIdx >= 0 && cursorLineIdx < len(lines) {
		task, _ := m.getTaskRef()
		if task != nil {
			indent := "      " // 6 spaces for single-project task line alignment
			pLines := inlinePreviewLines(task, indent, m.Width())
			if len(pLines) > 0 {
				insertAt := cursorLineIdx + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
					contentHeight -= len(pLines)
					if contentHeight < 1 {
						contentHeight = 1
					}
				}
			}
		}
	}

	visibleLines, startIdx, endIdx := persistentLineWindow(lines, cursorLineIdx, contentHeight, &m.viewScrollOffset)

	// Tab bar
	b.WriteString(m.renderPriorityTabBar())
	b.WriteString("\n")

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
	}

	// Filter indicator
	if filterIndicator != "" {
		b.WriteString(filterIndicator)
		b.WriteString("\n")
	}

	// Suggestion section
	if suggestionSection != "" {
		b.WriteString(suggestionSection)
		b.WriteString("\n")
	}

	// Visible task lines
	visibleContent := strings.Join(visibleLines, "\n")
	scrollbar := RenderScrollbar(len(lines), endIdx-startIdx, startIdx)
	contentWidth := m.Width()
	if scrollbar != "" {
		contentWidth = max(contentWidth-1, 1)
	}
	b.WriteString(attachScrollbarAtWidth(visibleContent, scrollbar, contentWidth))
	if stripTask != nil {
		strip := renderTaskMetadataStrip(TaskMetadataStripOpts{
			Task:    stripTask,
			Project: stripProject,
			Runtime: m.runtime,
			Width:   m.Width(),
			AlreadyShown: TaskMetadataAlreadyShown{
				Project:  true,
				Priority: true,
				Score:    true,
				Due:      stripTask.DueDate != nil,
			},
		})
		b.WriteString("\n")
		b.WriteString(strip)
	}
	b.WriteString("\n")

	// Search bar (shown above footer when active)
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render("Search: ") + m.SearchInputView())
		b.WriteString("\n")
	}
	left := HelpStyle.Render("↑/↓ navigate · / search · shift+←/→ tabs · F filter · s start suggested · ,s/,d status" + tabHint)
	if m.multiSelectMode {
		left = HelpStyle.Render("MULTI-SELECT: space toggle · a all · A deselect · ,/! status/priority · v exit")
	}
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// GetHelpItems returns help items for this view.
func (m *PriorityViewModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *PriorityViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *PriorityViewModel) StatusBarLabel() string { return "Priority" }

// IsFullHeight returns true because the priority view renders its own footer.
func (m *PriorityViewModel) IsFullHeight() bool { return true }

// Multi-select helpers.

func (m *PriorityViewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *PriorityViewModel) toggleTaskSelection() {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return
	}
	m.ToggleAndUpdateStatus(pt.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *PriorityViewModel) selectAllVisible() {
	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()
	for _, group := range [][]*PriorityTask{overdue, today, thisWeek, later, backlog, waitingOn} {
		for _, pt := range group {
			m.selectedTasks[pt.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *PriorityViewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}
