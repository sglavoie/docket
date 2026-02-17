package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	InteractiveViewState

	project        *models.Project
	config         *config.Config
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
	activeTab int // Index into priorityTabNames

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

	// Navigation
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Filter state
	filter           PriorityFilterState
	filterBar        *PriorityFilterBarComponent
	actionableFilter bool // hide blocked tasks even in All tab

	// Waiting On section state
	waitingOnCollapsed bool

	// Status and error messages
	err error

	// Dimensions
	width  int
	height int
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project, cfg ...*config.Config) *PriorityViewModel {
	m := &PriorityViewModel{
		project:            project,
		projectService:     NewProjectService(),
		waitingOnCollapsed: true,
		dismissedIDs:       make(map[string]bool),
		filter: PriorityFilterState{
			CommonFilterState: CommonFilterState{
				Tags:  make(map[string]bool),
				Attrs: make(map[string]string),
			},
		},
		filterBar: NewPriorityFilterBarComponent(),
	}
	if len(cfg) > 0 {
		m.config = cfg[0]
	}
	m.loadActionableTasks()
	return m
}

// HasActiveSearch returns false (no search in this view).
func (m *PriorityViewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *PriorityViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *PriorityViewModel) Init() tea.Cmd {
	return nil
}

func (m *PriorityViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.loadActionableTasks()
			m.clampCursor()
			m.statusMessage = "Tags updated"
		}
		return m, nil

	case SnoozeDateChangedMsg:
		m.loadActionableTasks()
		m.clampCursor()
		m.statusMessage = "Snooze date updated"
		return m, nil

	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}

	return m, nil
}

// HasTextInput returns false for the priority view.
func (m *PriorityViewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when the priority view is in prefix mode or confirm dialog is active.
func (m *PriorityViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.confirm.IsActive()
}

func (m *PriorityViewModel) View() string {
	var b strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks • %s", m.totalTasks, m.project.Name)
	b.WriteString(renderFullscreenTitle("Priority View", subtitle))
	b.WriteString("\n")

	_, _, _, _, _, filteredWaiting := m.filteredGroupsWithSubTab()

	if m.totalTasks == 0 && len(filteredWaiting) == 0 {
		b.WriteString(m.renderPriorityTabBar())
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("No tasks in this view."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[/]: tabs" + m.activeTabHint()))
		return b.String()
	}

	// Pre-render conditional sections to calculate their actual height
	suggestionSection := ""
	if m.activeTab != priorityTabByScore {
		suggestionSection = m.renderSuggestionSection()
	}
	tabHint := m.activeTabHint()
	filterIndicator := m.renderActiveFilterIndicator()

	// Calculate available height for task list
	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + footer(2) = 6
	contentHeight := m.height - 6

	// Account for sub-tab bar
	hasSubTabs := len(m.subTabNames) > 1
	if hasSubTabs {
		contentHeight -= 2 // sub-tab row + newline
	}

	// Account for filter indicator if present
	if filterIndicator != "" {
		contentHeight-- // filter badge + newline
	}

	// Account for suggestion section if present
	if suggestionSection != "" {
		suggestionLines := lipgloss.Height(suggestionSection)
		contentHeight -= suggestionLines
	}

	// Render groups
	currentIndex := 0
	lines := m.renderAllGroups(&currentIndex)

	// Reserve line for scroll indicator if we'll need to scroll
	needsScrolling := len(lines) > contentHeight
	if needsScrolling {
		contentHeight--
	}

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Apply centered scrolling (keep cursor in middle of viewport)
	startIdx := 0
	endIdx := len(lines)

	if len(lines) > contentHeight {
		halfView := contentHeight / 2
		startIdx = m.cursor - halfView
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + contentHeight
		if endIdx > len(lines) {
			endIdx = len(lines)
			startIdx = endIdx - contentHeight
			if startIdx < 0 {
				startIdx = 0
			}
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
	b.WriteString(strings.Join(visibleLines, "\n"))

	// Scroll indicator
	if needsScrolling {
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", startIdx+1, endIdx, len(lines))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(scrollInfo))
	}
	b.WriteString("\n")

	// Footer
	left := HelpStyle.Render("↑/↓ navigate • [/] tabs • F filter • s start suggested • ,s/,d status" + tabHint)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// GetHelpItems returns help items for this view.
func (m *PriorityViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewPriorityView)
}
