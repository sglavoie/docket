package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
var priorityTabNames = []string{"All", "Urgent", "Upcoming", "Backlog", "Blocked"}

const (
	priorityTabAll      = 0
	priorityTabUrgent   = 1
	priorityTabUpcoming = 2
	priorityTabBacklog  = 3
	priorityTabBlocked  = 4
)

// PriorityViewModel displays actionable tasks sorted by urgency.
type PriorityViewModel struct {
	project        *models.Project
	projectService *ProjectService

	// Grouped tasks by urgency
	overdue   []*PriorityTask
	today     []*PriorityTask
	thisWeek  []*PriorityTask
	later     []*PriorityTask
	backlog   []*PriorityTask // Tasks without due dates
	waitingOn []*PriorityTask // Blocked tasks

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
	cursor      int   // Absolute position across all visible groups
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Waiting On section state
	waitingOnCollapsed bool

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "g" when waiting for second key

	// Dimensions
	width  int
	height int
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project) *PriorityViewModel {
	m := &PriorityViewModel{
		project:            project,
		projectService:     NewProjectService(),
		waitingOnCollapsed: true,
		dismissedIDs:       make(map[string]bool),
	}
	m.loadActionableTasks()
	return m
}

// loadActionableTasks categorizes and sorts tasks by urgency.
func (m *PriorityViewModel) loadActionableTasks() {
	m.overdue = nil
	m.today = nil
	m.thisWeek = nil
	m.later = nil
	m.backlog = nil
	m.waitingOn = nil

	for _, task := range m.project.Tasks {
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}

		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}

		// Check if blocked (status=blocked or has incomplete dependencies)
		isBlocked := task.Status == models.StatusBlocked || m.project.HasIncompleteDependencies(task)

		pt := &PriorityTask{
			Task:         task,
			UrgencyScore: m.computeUrgencyScore(task),
			ParentTitle:  m.getParentTitle(task),
			RelativeDue:  m.formatRelativeDue(task),
		}

		if isBlocked {
			m.waitingOn = append(m.waitingOn, pt)
			continue
		}

		// Categorize by due date
		group := m.categorizeByDueDate(task)
		switch group {
		case PriorityGroupOverdue:
			m.overdue = append(m.overdue, pt)
		case PriorityGroupToday:
			m.today = append(m.today, pt)
		case PriorityGroupThisWeek:
			m.thisWeek = append(m.thisWeek, pt)
		case PriorityGroupLater:
			m.later = append(m.later, pt)
		case PriorityGroupBacklog:
			m.backlog = append(m.backlog, pt)
		}
	}

	// Sort each group by urgency score (higher = more urgent)
	sortByUrgency := func(tasks []*PriorityTask) {
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].UrgencyScore > tasks[j].UrgencyScore
		})
	}
	sortByUrgency(m.overdue)
	sortByUrgency(m.today)
	sortByUrgency(m.thisWeek)
	sortByUrgency(m.later)
	sortByUrgency(m.backlog)
	sortByUrgency(m.waitingOn)

	// Recalculate navigation for the active tab
	m.recalculateNavigation()

	// Compute the smart suggestion
	m.computeSuggestion()
}

// activeTabHint returns a brief description of the active tab's purpose for the footer.
func (m *PriorityViewModel) activeTabHint() string {
	switch m.activeTab {
	case priorityTabUrgent:
		return " • Showing overdue and due today"
	case priorityTabUpcoming:
		return " • Showing this week and later"
	case priorityTabBacklog:
		return " • Showing tasks with no due date"
	case priorityTabBlocked:
		return " • Showing blocked and waiting tasks"
	default:
		return ""
	}
}

// filteredGroups returns the task groups visible for the current tab.
// Returns overdue, today, thisWeek, later, backlog, waitingOn slices filtered by active tab.
func (m *PriorityViewModel) filteredGroups() (overdue, today, thisWeek, later, backlog, waitingOn []*PriorityTask) {
	switch m.activeTab {
	case priorityTabUrgent:
		return m.overdue, m.today, nil, nil, nil, nil
	case priorityTabUpcoming:
		return nil, nil, m.thisWeek, m.later, nil, nil
	case priorityTabBacklog:
		return nil, nil, nil, nil, m.backlog, nil
	case priorityTabBlocked:
		return nil, nil, nil, nil, nil, m.waitingOn
	default: // priorityTabAll
		return m.overdue, m.today, m.thisWeek, m.later, m.backlog, m.waitingOn
	}
}

// computeSubTabs builds dynamic sub-tab names based on active tab and current groups.
func (m *PriorityViewModel) computeSubTabs() {
	if m.project == nil || m.activeTab == priorityTabAll {
		m.subTabNames = nil
		if m.subTab > len(m.subTabNames) {
			m.subTab = 0
		}
		return
	}

	// Collect tasks visible for the current tab
	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroups()
	var tasks []*PriorityTask
	tasks = append(tasks, overdue...)
	tasks = append(tasks, today...)
	tasks = append(tasks, thisWeek...)
	tasks = append(tasks, later...)
	tasks = append(tasks, backlog...)
	tasks = append(tasks, waitingOn...)

	// Extract unique custom sections from these tasks
	seen := map[string]bool{}
	var names []string
	for _, pt := range tasks {
		section := pt.Task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if !seen[section] {
			seen[section] = true
			names = append(names, section)
		}
	}
	sort.Strings(names)
	m.subTabNames = names

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *PriorityViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// filterBySubTab narrows a task slice by the active sub-tab custom section filter.
func (m *PriorityViewModel) filterBySubTab(tasks []*PriorityTask) []*PriorityTask {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return tasks
	}
	var result []*PriorityTask
	for _, pt := range tasks {
		section := pt.Task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if section == filter {
			result = append(result, pt)
		}
	}
	return result
}

// filteredGroupsWithSubTab returns filtered groups further narrowed by the active sub-tab.
func (m *PriorityViewModel) filteredGroupsWithSubTab() (overdue, today, thisWeek, later, backlog, waitingOn []*PriorityTask) {
	o, td, tw, l, bl, wo := m.filteredGroups()
	return m.filterBySubTab(o), m.filterBySubTab(td), m.filterBySubTab(tw), m.filterBySubTab(l), m.filterBySubTab(bl), m.filterBySubTab(wo)
}

// recalculateNavigation updates totalTasks and groupStarts based on the active tab.
func (m *PriorityViewModel) recalculateNavigation() {
	m.computeSubTabs()
	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, waiting tasks are navigable (not collapsible)
	if m.activeTab == priorityTabBlocked {
		m.totalTasks = len(waitingOn)
		m.groupStarts = []int{0}
	} else {
		m.totalTasks = len(overdue) + len(today) + len(thisWeek) + len(later) + len(backlog)
		m.groupStarts = []int{
			0,
			len(overdue),
			len(overdue) + len(today),
			len(overdue) + len(today) + len(thisWeek),
			len(overdue) + len(today) + len(thisWeek) + len(later),
		}
	}
}

// jumpToTask moves the cursor to the position of the given task in the list.
func (m *PriorityViewModel) jumpToTask(task *models.Task) {
	index := 0
	for _, pt := range m.overdue {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.today {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.thisWeek {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.later {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.backlog {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
}

// getParentTitle returns the parent task's title for context.
func (m *PriorityViewModel) getParentTitle(task *models.Task) string {
	if task.ParentID == nil {
		return ""
	}
	if parent := m.project.GetTaskByID(*task.ParentID); parent != nil {
		return parent.Title
	}
	return ""
}

// formatRelativeDue formats the due date for display.
func (m *PriorityViewModel) formatRelativeDue(task *models.Task) string {
	if task.DueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return task.DueDate.Format("Mon")
	default:
		if task.DueDate.Year() != now.Year() {
			return task.DueDate.Format("Jan 2 '06")
		}
		return task.DueDate.Format("Jan 2")
	}
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *PriorityViewModel) getTaskAtCursor() *PriorityTask {
	if m.totalTasks == 0 {
		return nil
	}

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// For the Blocked tab, navigate waiting tasks directly
	if m.activeTab == priorityTabBlocked {
		if m.cursor < len(waitingOn) {
			return waitingOn[m.cursor]
		}
		return nil
	}

	cursor := m.cursor
	if cursor < len(overdue) {
		return overdue[cursor]
	}
	cursor -= len(overdue)
	if cursor < len(today) {
		return today[cursor]
	}
	cursor -= len(today)
	if cursor < len(thisWeek) {
		return thisWeek[cursor]
	}
	cursor -= len(thisWeek)
	if cursor < len(later) {
		return later[cursor]
	}
	cursor -= len(later)
	if cursor < len(backlog) {
		return backlog[cursor]
	}
	return nil
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

	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}

	return m, nil
}

func (m *PriorityViewModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (Vim-style gg)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Tab row focus switching
	switch key {
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil
	case "[", "h":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab > 0 {
				m.subTab--
			} else {
				m.subTab = total - 1
			}
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(priorityTabNames) - 1
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	case "]", "l":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab < total-1 {
				m.subTab++
			} else {
				m.subTab = 0
			}
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			if m.activeTab < len(priorityTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return ClosePriorityViewMsg{}
		}

	case "enter":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: pt.Task}
			}
		}
		return m, nil

	// Navigation
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < m.totalTasks-1 {
			m.cursor++
		}
		return m, nil

	case ",":
		// Start prefix sequence for status operations
		m.pendingPrefix = ","
		return m, nil

	case "g":
		// Start prefix sequence for gg (Vim-style)
		m.pendingPrefix = "g"
		return m, nil

	case "d":
		// Start prefix sequence for dd (due date)
		m.pendingPrefix = "d"
		return m, nil

	case "c":
		// Start prefix sequence for create linked task
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "c"
		}
		return m, nil

	case "G":
		if m.totalTasks > 0 {
			m.cursor = m.totalTasks - 1
		}
		return m, nil

	case "ctrl+u":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	case "ctrl+d":
		m.cursor += 10
		if m.cursor >= m.totalTasks {
			m.cursor = m.totalTasks - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	// Toggle waiting on section
	case "tab":
		m.waitingOnCollapsed = !m.waitingOnCollapsed
		return m, nil

	// Priority changes
	case "1":
		return m.changeTaskPriority(1)
	case "2":
		return m.changeTaskPriority(2)
	case "3":
		return m.changeTaskPriority(3)
	case "4":
		return m.changeTaskPriority(4)
	case "5":
		return m.changeTaskPriority(5)

	case "e":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "y":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "Y":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "ctrl+y":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewPriorityView,
				ViewName:   "Priority View",
				Items:      m.GetHelpItems(),
			}
		}

	case "r":
		// Refresh: reload project and clear dismissed suggestions
		_ = m.projectService.Load(m.project)
		m.dismissedIDs = make(map[string]bool)
		m.loadActionableTasks()
		m.clampCursor()
		return m, nil

	// Suggestion actions
	case "s":
		// Start suggested task (or jump to it if already in progress)
		return m.startSuggestedTask()

	case "x":
		// Dismiss current suggestion
		m.dismissSuggestion()
		return m, nil
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *PriorityViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
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
			return m.changeTaskStatus(models.StatusInProgress)
		case "d":
			return m.changeTaskStatus(models.StatusDone)
		case "b":
			return m.changeTaskStatus(models.StatusBlocked)
		case "o":
			return m.changeTaskStatus(models.StatusTodo)
		case "x":
			return m.changeTaskStatus(models.StatusCanceled)
		}
	case "g":
		switch key {
		case "g":
			// gg: Go to first item
			m.cursor = 0
			return m, nil
		case "t":
			// gt: Go to task in project tree
			if pt := m.getTaskAtCursor(); pt != nil {
				return m, func() tea.Msg {
					return PriorityViewSelectMsg{Task: pt.Task}
				}
			}
			return m, nil
		case "h":
			// gh: Go to task activity history
			if pt := m.getTaskAtCursor(); pt != nil {
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: pt.Task, Project: project}
				}
			}
			return m, nil
		}
	case "d":
		switch key {
		case "d":
			// dd: Open date picker for task at cursor
			if pt := m.getTaskAtCursor(); pt != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: pt.Task, Project: m.project}
				}
			}
			return m, nil
		}
	case "c":
		// Create linked task or navigate
		pt := m.getTaskAtCursor()
		if pt == nil {
			return m, nil
		}
		task := pt.Task
		project := m.project

		// Navigate to linked tasks
		if key == "n" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
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

	// Invalid prefix sequence - ignore
	return m, nil
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *PriorityViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false for the priority view.
func (m *PriorityViewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when the priority view is in prefix mode.
func (m *PriorityViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix()
}

// changeTaskStatus changes the status of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	task := pt.Task
	if task.Status == newStatus {
		return m, nil
	}

	// Backup before change
	if err := m.projectService.Backup(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
	}

	// Update task status
	task.Status = newStatus
	task.UpdatedAt = time.Now()
	if newStatus.IsComplete() {
		now := time.Now()
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}

	// Save project
	if err := m.projectService.Save(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	// Reload and refresh
	_ = m.projectService.Load(m.project)
	m.loadActionableTasks()
	m.clampCursor()

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	task := pt.Task
	if task.Priority == newPriority {
		return m, nil
	}

	// Backup before change
	if err := m.projectService.Backup(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
	}

	// Update task priority
	task.Priority = newPriority
	task.UpdatedAt = time.Now()

	// Save project
	if err := m.projectService.Save(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	// Reload and refresh
	_ = m.projectService.Load(m.project)
	m.loadActionableTasks()
	m.clampCursor()

	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *PriorityViewModel) clampCursor() {
	if m.cursor >= m.totalTasks {
		m.cursor = m.totalTasks - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
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
	suggestionSection := m.renderSuggestionSection()
	prefixPopup := m.renderPrefixPopup()
	tabHint := m.activeTabHint()

	// Calculate available height for task list
	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + footer(2) = 6
	contentHeight := m.height - 6

	// Account for sub-tab bar
	hasSubTabs := len(m.subTabNames) > 1
	if hasSubTabs {
		contentHeight -= 2 // sub-tab row + newline
	}

	// Account for suggestion section if present
	if suggestionSection != "" {
		suggestionLines := lipgloss.Height(suggestionSection)
		contentHeight -= suggestionLines
	}

	// Account for prefix popup if present
	if prefixPopup != "" {
		popupLines := lipgloss.Height(prefixPopup) + 1
		contentHeight -= popupLines
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

	// Prefix popup
	if prefixPopup != "" {
		b.WriteString(prefixPopup)
		b.WriteString("\n")
	}

	// Footer
	left := HelpStyle.Render("↑/↓ navigate • [/] tabs • s start suggested • ,s/,d status" + tabHint)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// GetHelpItems returns help items for this view.
func (m *PriorityViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewPriorityView)
}
