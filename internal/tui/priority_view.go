package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
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
	cursor      int   // Absolute position across all visible groups
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Filter state
	filter           PriorityFilterState
	filterBar        *PriorityFilterBarComponent
	actionableFilter bool // hide blocked tasks even in All tab

	// Waiting On section state
	waitingOnCollapsed bool

	// Status and error messages
	statusMessage string
	err           error

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "g" when waiting for second key

	// Confirm dialog for destructive actions
	confirm ConfirmDialog

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
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
		filterBar: NewPriorityFilterBarComponent(),
	}
	if len(cfg) > 0 {
		m.config = cfg[0]
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

	// Apply sub-filters to each group
	m.overdue = m.applyFiltersToGroup(m.overdue)
	m.today = m.applyFiltersToGroup(m.today)
	m.thisWeek = m.applyFiltersToGroup(m.thisWeek)
	m.later = m.applyFiltersToGroup(m.later)
	m.backlog = m.applyFiltersToGroup(m.backlog)
	m.waitingOn = m.applyFiltersToGroup(m.waitingOn)

	// Compute score-ranked list if a scoring profile is assigned
	m.scoredTasks = nil
	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			var all []*PriorityTask
			all = append(all, m.overdue...)
			all = append(all, m.today...)
			all = append(all, m.thisWeek...)
			all = append(all, m.later...)
			all = append(all, m.backlog...)
			sort.SliceStable(all, func(i, j int) bool {
				return scoring.ComputeScore(all[i].Task, profile) > scoring.ComputeScore(all[j].Task, profile)
			})
			m.scoredTasks = all
		}
	}

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
	case priorityTabByScore:
		return " • Ranked by composite score"
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
	case priorityTabByScore:
		return nil, nil, nil, nil, nil, nil
	default: // priorityTabAll
		return m.overdue, m.today, m.thisWeek, m.later, m.backlog, m.waitingOn
	}
}

// computeSubTabs builds dynamic sub-tab names based on active tab and current groups.
func (m *PriorityViewModel) computeSubTabs() {
	if m.project == nil || m.activeTab == priorityTabAll || m.activeTab == priorityTabByScore {
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

	// By Score tab uses scoredTasks directly
	if m.activeTab == priorityTabByScore {
		m.subTabNames = nil
		m.totalTasks = len(m.scoredTasks)
		m.groupStarts = []int{0}
		return
	}

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
	if m.activeTab == priorityTabByScore {
		for i, pt := range m.scoredTasks {
			if pt.Task.ID == task.ID {
				m.cursor = i
				return
			}
		}
		return
	}
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

	if m.activeTab == priorityTabByScore {
		if m.cursor < len(m.scoredTasks) {
			return m.scoredTasks[m.cursor]
		}
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

func (m *PriorityViewModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (Vim-style gg)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirm dialog
	if m.confirm.IsActive() {
		confirmed, handled := m.confirm.HandleKey(key)
		if handled {
			if confirmed {
				return m.deleteTask()
			}
			m.statusMessage = ""
			return m, nil
		}
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
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
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

	case "m":
		// Start prefix sequence for section move
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "m"
		}
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

	case "S":
		// Open score editor for task under cursor
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: pt.Task, Project: m.project}
			}
		}
		return m, nil

	case "x":
		// Dismiss current suggestion
		m.dismissSuggestion()
		return m, nil

	case "F":
		// Open filter prefix popup
		m.pendingPrefix = "F"
		return m, nil

	// Tags
	case "t":
		if pt := m.getTaskAtCursor(); pt != nil {
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: append([]string{}, task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if pt := m.getTaskAtCursor(); pt != nil {
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: task, Project: project}
			}
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if pt := m.getTaskAtCursor(); pt != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", pt.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
		}
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
		case "r":
			return m.changeTaskStatus(models.StatusReview)
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
		case "t":
			return m.setQuickDueDate(0) // Today
		case "m":
			return m.setQuickDueDate(1) // Tomorrow
		case "w":
			return m.setQuickDueDate(7) // +1 week
		case "n":
			return m.setQuickDueDate(30) // +1 month
		case "0":
			return m.clearDueDate()
		case "d":
			// dd: Open date picker for task at cursor
			if pt := m.getTaskAtCursor(); pt != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: pt.Task, Project: m.project}
				}
			}
			return m, nil
		}
	case "m":
		section, ok := sectionForMoveKey(key)
		if !ok {
			return m, nil
		}
		return m.sectionMove(section)
	case "F":
		// Filter prefix - first level
		return m.handlePriorityFilterPrefix(key)
	case "Ft":
		return m.handlePriorityFilterTypePrefix(key)
	case "Fs":
		return m.handlePriorityFilterStatusPrefix(key)
	case "Fd":
		return m.handlePriorityFilterDuePrefix(key)
	case "Fa":
		return m.handlePriorityFilterAttrPrefix(key)
	case "Fas":
		return m.handlePriorityFilterAttrSeverityPrefix(key)
	case "Far":
		return m.handlePriorityFilterAttrReproduciblePrefix(key)
	case "Fae":
		return m.handlePriorityFilterAttrEffortPrefix(key)
	case "Fac":
		return m.handlePriorityFilterAttrCategoryPrefix(key)
	case "Fav":
		return m.handlePriorityFilterAttrValuePrefix(key)
	case "Faw":
		return m.handlePriorityFilterAttrWorktypePrefix(key)
	case "Fao":
		return m.handlePriorityFilterAttrScopePrefix(key)
	case "Fap":
		return m.handlePriorityFilterAttrPointsPrefix(key)
	case "FS":
		return m.handlePriorityFilterScorePrefix(key)
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

// ShouldCaptureEsc returns true when the priority view is in prefix mode or confirm dialog is active.
func (m *PriorityViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.confirm.IsActive()
}

// changeTaskStatus changes the status of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyStatusChangePS(m.projectService, m.project, pt.Task.ID, newStatus)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// changeTaskPriority changes the priority of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyPriorityChangePS(m.projectService, m.project, pt.Task.ID, newPriority)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *PriorityViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyQuickDueDatePS(m.projectService, m.project, pt.Task.ID, daysFromNow)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// clearDueDate clears the due date from the task at cursor.
func (m *PriorityViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyClearDueDatePS(m.projectService, m.project, pt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// toggleActive toggles the Active flag on the task at cursor.
func (m *PriorityViewModel) toggleActive() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyToggleActivePS(m.projectService, m.project, pt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// sectionMove moves the task at cursor to the specified GTD section.
func (m *PriorityViewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applySectionChangePS(m.projectService, m.project, pt.Task.ID, section)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
}

// deleteTask deletes the task at cursor.
func (m *PriorityViewModel) deleteTask() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	result := applyDeleteTaskPS(m.projectService, m.project, pt.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadActionableTasks()
	m.clampCursor()
	return m, result.Cmd
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
