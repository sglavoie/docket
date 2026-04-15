package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	ViewBundle

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

	// Waiting On section state
	waitingOnCollapsed bool

	// Inline preview
	previewMode bool
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project, cfg *config.Config, mgr *config.Manager) *PriorityViewModel {
	m := &PriorityViewModel{
		ViewBundle:         ViewBundle{config: cfg, configManager: mgr},
		project:            project,
		projectService:     NewProjectService(),
		waitingOnCollapsed: true,
		dismissedIDs:       make(map[string]bool),
		FilterMixin:        FilterMixin{filter: newFilterState()},
		tabs:               TabState{Names: priorityTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Search tasks...", 100, 30)
	m.loadActionableTasks()
	return m
}

func (m *PriorityViewModel) Init() tea.Cmd {
	return nil
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
		b.WriteString(HelpStyle.Render("[/]: tabs" + m.activeTabHint()))
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

	// Apply centered scrolling (keep cursor in middle of viewport)
	startIdx := 0
	endIdx := len(lines)

	if len(lines) > contentHeight {
		halfView := contentHeight / 2
		startIdx = max(cursorLineIdx-halfView, 0)
		endIdx = startIdx + contentHeight
		if endIdx > len(lines) {
			endIdx = len(lines)
			startIdx = max(endIdx-contentHeight, 0)
		}
	}

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
	visibleLines := lines[startIdx:endIdx]
	visibleContent := strings.Join(visibleLines, "\n")
	scrollbar := RenderScrollbar(len(lines), endIdx-startIdx, startIdx)
	b.WriteString(AttachScrollbar(visibleContent, scrollbar))
	b.WriteString("\n")

	// Search bar (shown above footer when active)
	if m.IsSearchMode() {
		b.WriteString(HelpStyle.Render("Search: ") + m.SearchInputView())
		b.WriteString("\n")
	}
	left := HelpStyle.Render("↑/↓ navigate · / search · [/] tabs · F filter · s start suggested · ,s/,d status" + tabHint)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// GetHelpItems returns help items for this view.
func (m *PriorityViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewPriorityView)
}

func (m *PriorityViewModel) HandledKeys() []string {
	return concatKeys(
		listNavigationKeys(),
		statusPrefixKeys(),
		priorityPrefixKeys(),
		dueDatePrefixKeys(),
		snoozePrefixKeys(),
		linkedTaskPrefixKeys(),
		stagePrefixKeys(),
		commonActionKeys(),
		[]string{
			"g", "gt",
			"w", "x", "A",
			"enter", "tab", "F", "m", "t", "/",
			"1", "2", "3", "4", "5",
			"esc",
		},
	)
}

func (m *PriorityViewModel) StatusBarLabel() string { return "Priority" }

// IsFullHeight returns true because the priority view renders its own footer.
func (m *PriorityViewModel) IsFullHeight() bool { return true }
