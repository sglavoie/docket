package tui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// AllTasksItem wraps a task with its parent project for cross-project display.
type AllTasksItem struct {
	Task    *models.Task
	Project *models.Project
}

// AllTasksGroupBy represents the grouping strategy for the all tasks view.
type AllTasksGroupBy int

const (
	ATGroupByProject  AllTasksGroupBy = iota
	ATGroupByStatus                   // by workflow status
	ATGroupByPriority                 // by priority level
	ATGroupBySection                  // by GTD section
	ATGroupByDueDate                  // by due date bucket
	ATGroupByFlat                     // no grouping
)

func (g AllTasksGroupBy) String() string {
	switch g {
	case ATGroupByProject:
		return "Project"
	case ATGroupByStatus:
		return "Status"
	case ATGroupByPriority:
		return "Priority"
	case ATGroupBySection:
		return "Section"
	case ATGroupByDueDate:
		return "Due Date"
	case ATGroupByFlat:
		return "Flat"
	default:
		return "Project"
	}
}

// AllTasksGroup represents a named group of tasks.
type AllTasksGroup struct {
	Name  string
	Tasks []AllTasksItem
}

var allTasksTabNames = []string{"All", "Active", "Due Soon", "High Pri", "Blocked", "By Score"}

const (
	allTasksTabAll     = 0
	allTasksTabActive  = 1
	allTasksTabDueSoon = 2
	allTasksTabHighPri = 3
	allTasksTabBlocked = 4
	allTasksTabByScore = 5
)

// AllTasksViewModel displays all tasks across all projects.
type AllTasksViewModel struct {
	config         *config.Config
	projectService *ProjectService
	projectCache   map[string]*models.Project

	// All loaded tasks (unfiltered)
	allTasks []AllTasksItem

	// Grouped/filtered view
	groups     []AllTasksGroup
	totalItems int

	// Grouping
	groupBy         AllTasksGroupBy
	collapsedGroups map[string]bool

	// Tabs
	activeTab int

	// Filter
	filter        CommonFilterState
	projectFilter map[string]bool // included project IDs (empty = all)

	// Sort
	sortField     SortField
	sortAscending bool

	// Multi-select
	multiSelectMode bool
	selectedTasks   map[string]bool

	// Navigation
	cursor       int
	scrollOffset int
	groupStarts  []int // cursor positions of group headers

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string

	// UI feedback
	statusMessage string
	err           error
	confirm       ConfirmDialog

	// Dimensions
	width  int
	height int
}

// NewAllTasksViewModel creates a new all tasks view model.
func NewAllTasksViewModel(cfg *config.Config) *AllTasksViewModel {
	m := &AllTasksViewModel{
		config:          cfg,
		projectService:  NewProjectService(),
		projectCache:    make(map[string]*models.Project),
		collapsedGroups: make(map[string]bool),
		projectFilter:   make(map[string]bool),
		selectedTasks:   make(map[string]bool),
		sortField:       SortByDefault,
		sortAscending:   true,
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
	}
	m.loadAllTasks()
	return m
}

// loadAllTasks loads tasks from all active projects.
func (m *AllTasksViewModel) loadAllTasks() {
	m.allTasks = nil
	m.projectCache = make(map[string]*models.Project)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		m.projectCache[project.ID] = project

		for _, task := range project.Tasks {
			// Skip completed/canceled by default
			if task.Status.IsComplete() {
				continue
			}
			m.allTasks = append(m.allTasks, AllTasksItem{Task: task, Project: project})
		}
	}

	m.applyFiltersAndGroup()
}

// applyFiltersAndGroup applies filters, tab filter, grouping, and sorting.
func (m *AllTasksViewModel) applyFiltersAndGroup() {
	// Step 1: Apply common filters
	filtered := m.allTasks
	if hasActiveCommonFilters(&m.filter) {
		filtered = nil
		for _, item := range m.allTasks {
			if passesCommonFilters(item.Task, &m.filter, item.Project.ID, m.config) {
				filtered = append(filtered, item)
			}
		}
	}

	// Step 2: Apply project filter
	if len(m.projectFilter) > 0 {
		var projectFiltered []AllTasksItem
		for _, item := range filtered {
			if m.projectFilter[item.Project.ID] {
				projectFiltered = append(projectFiltered, item)
			}
		}
		filtered = projectFiltered
	}

	// Step 3: Apply tab-specific filter
	filtered = m.applyTabFilter(filtered)

	// Step 4: Sort
	m.applySorting(filtered)

	// Step 5: Group
	m.groups = m.groupTasks(filtered)

	// Step 6: Calculate navigation (headers + visible tasks)
	m.calculateNavigation()
}

// applyTabFilter filters tasks based on the active tab.
func (m *AllTasksViewModel) applyTabFilter(tasks []AllTasksItem) []AllTasksItem {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch m.activeTab {
	case allTasksTabActive:
		var result []AllTasksItem
		for _, item := range tasks {
			if item.Task.Status == models.StatusInProgress {
				result = append(result, item)
			}
		}
		return result

	case allTasksTabDueSoon:
		sevenDaysLater := today.AddDate(0, 0, 7)
		var result []AllTasksItem
		for _, item := range tasks {
			if item.Task.DueDate == nil {
				continue
			}
			dueDay := time.Date(item.Task.DueDate.Year(), item.Task.DueDate.Month(), item.Task.DueDate.Day(), 0, 0, 0, 0, now.Location())
			if dueDay.Before(sevenDaysLater) {
				result = append(result, item)
			}
		}
		return result

	case allTasksTabHighPri:
		var result []AllTasksItem
		for _, item := range tasks {
			if item.Task.Priority <= 2 {
				result = append(result, item)
			}
		}
		return result

	case allTasksTabBlocked:
		var result []AllTasksItem
		for _, item := range tasks {
			if item.Task.Status == models.StatusBlocked {
				result = append(result, item)
			}
		}
		return result

	case allTasksTabByScore:
		// Only tasks with scores; sorted by score descending
		var result []AllTasksItem
		for _, item := range tasks {
			profile := m.config.GetProjectScoringProfile(item.Project.ID)
			if profile != nil && HasManualScores(item.Task, profile) {
				result = append(result, item)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			pi := m.config.GetProjectScoringProfile(result[i].Project.ID)
			pj := m.config.GetProjectScoringProfile(result[j].Project.ID)
			si := scoring.ComputeScore(result[i].Task, pi)
			sj := scoring.ComputeScore(result[j].Task, pj)
			return si > sj
		})
		return result
	}

	return tasks
}

// calculateNavigation counts navigable items (headers + visible tasks)
// and builds groupStarts for n/N header jumping.
func (m *AllTasksViewModel) calculateNavigation() {
	m.totalItems = 0
	m.groupStarts = nil
	for _, group := range m.groups {
		m.groupStarts = append(m.groupStarts, m.totalItems)
		m.totalItems++ // header
		if !m.collapsedGroups[group.Name] {
			m.totalItems += len(group.Tasks)
		}
	}
}

// getTaskAtCursor returns the task at the current cursor position.
// Returns nil if cursor is on a group header.
func (m *AllTasksViewModel) getTaskAtCursor() *AllTasksItem {
	pos := 0
	for i := range m.groups {
		if m.cursor == pos {
			return nil // On header
		}
		pos++ // past header

		if !m.collapsedGroups[m.groups[i].Name] {
			for j := range m.groups[i].Tasks {
				if m.cursor == pos {
					return &m.groups[i].Tasks[j]
				}
				pos++
			}
		}
	}
	return nil
}

// totalNavigableItems returns the count of navigable items (headers + visible tasks).
func (m *AllTasksViewModel) totalNavigableItems() int {
	return m.totalItems
}

// totalVisibleTaskCount returns the count of visible tasks (excluding headers).
func (m *AllTasksViewModel) totalVisibleTaskCount() int {
	count := 0
	for _, group := range m.groups {
		if !m.collapsedGroups[group.Name] {
			count += len(group.Tasks)
		}
	}
	return count
}

// getCurrentGroupIndex returns the index of the group containing the cursor.
func (m *AllTasksViewModel) getCurrentGroupIndex() int {
	for i := len(m.groupStarts) - 1; i >= 0; i-- {
		if m.cursor >= m.groupStarts[i] {
			return i
		}
	}
	return 0
}

// goToNextGroup moves cursor to next group header (with wrap).
func (m *AllTasksViewModel) goToNextGroup() (tea.Model, tea.Cmd) {
	if len(m.groupStarts) <= 1 {
		return m, nil
	}
	currentIdx := m.getCurrentGroupIndex()
	nextIdx := (currentIdx + 1) % len(m.groupStarts)
	m.cursor = m.groupStarts[nextIdx]
	return m, nil
}

// goToPrevGroup moves cursor to previous group header (with wrap).
func (m *AllTasksViewModel) goToPrevGroup() (tea.Model, tea.Cmd) {
	if len(m.groupStarts) <= 1 {
		return m, nil
	}
	currentIdx := m.getCurrentGroupIndex()
	prevIdx := currentIdx - 1
	if prevIdx < 0 {
		prevIdx = len(m.groupStarts) - 1
	}
	m.cursor = m.groupStarts[prevIdx]
	return m, nil
}

// projectCount returns the number of unique projects loaded.
func (m *AllTasksViewModel) projectCount() int {
	return len(m.projectCache)
}

// HasActiveSearch returns false (no search in this view).
func (m *AllTasksViewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *AllTasksViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *AllTasksViewModel) Init() tea.Cmd {
	return nil
}

func (m *AllTasksViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case AllTasksRefreshMsg:
		m.loadAllTasks()
		m.clampCursor()
		return m, nil

	case TaskUpdatedMsg:
		m.loadAllTasks()
		m.clampCursor()
		return m, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.loadAllTasks()
			m.clampCursor()
			m.statusMessage = "Tags updated"
		}
		return m, nil

	case SnoozeDateChangedMsg:
		m.loadAllTasks()
		m.clampCursor()
		m.statusMessage = "Snooze date updated"
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *AllTasksViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.statusMessage = ""

	// Handle confirm dialog
	if m.confirm.IsActive() {
		confirmed, handled := m.confirm.HandleKey(key)
		if handled {
			if confirmed {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m.handleBulkDelete()
				}
				return m.deleteTask()
			}
			m.statusMessage = ""
			return m, nil
		}
	}

	// Multi-select key handling
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case " ":
			m.toggleTaskSelection()
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAll()
			return m, nil
		case "D":
			if m.selectedCount() > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks? (y/n)", m.selectedCount()))
				m.statusMessage = m.confirm.Prompt()
			}
			return m, nil
		}
		// Allow prefix keys in multi-select for bulk operations
		switch key {
		case ",", "!", "d", "z":
			m.pendingPrefix = key
			return m, nil
		case "m":
			m.pendingPrefix = "m"
			return m, nil
		}
	}

	// Activate prefix mode
	switch key {
	case ",", "!", "d", "g", "z":
		m.pendingPrefix = key
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	case "m", "c":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.multiSelectMode {
			m.toggleMultiSelectMode()
			return m, nil
		}
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseAllTasksMsg{}
		}

	case "enter":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
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
		if m.cursor < m.totalNavigableItems()-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if m.totalNavigableItems() > 0 {
			m.cursor = m.totalNavigableItems() - 1
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
		if m.cursor >= m.totalNavigableItems() {
			m.cursor = m.totalNavigableItems() - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	// Group header navigation
	case "n":
		return m.goToNextGroup()
	case "N":
		return m.goToPrevGroup()

	// Tab navigation
	case "[", "h":
		if m.activeTab > 0 {
			m.activeTab--
		} else {
			m.activeTab = len(allTasksTabNames) - 1
		}
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil

	case "]", "l":
		if m.activeTab < len(allTasksTabNames)-1 {
			m.activeTab++
		} else {
			m.activeTab = 0
		}
		m.applyFiltersAndGroup()
		m.clampCursor()
		return m, nil

	// Group cycling
	case "ctrl+g":
		m.cycleGroupBy()
		return m, nil

	// Group collapse/expand
	case "tab":
		m.toggleGroupCollapse()
		return m, nil

	// Multi-select
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Toggle active flag
	case "*":
		return m.toggleActive()

	// Sort keys (1-5 for sort field, 0 to reset)
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
	case "5":
		m.cycleSortField(SortByScore)
		return m, nil
	case "0":
		m.sortField = SortByDefault
		m.sortAscending = true
		m.applyFiltersAndGroup()
		m.clampCursor()
		m.statusMessage = "Sort reset"
		return m, nil

	// Edit task
	case "e":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Refresh
	case "r":
		m.loadAllTasks()
		m.clampCursor()
		m.statusMessage = "Refreshed"
		return m, nil

	// Copy
	case "y":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: item.Task}
			}
		}
		return m, nil

	case "Y":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: item.Task}
			}
		}
		return m, nil

	case "ctrl+y":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: item.Task}
			}
		}
		return m, nil

	// Tags
	case "t":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: item.Task, Project: item.Project, CurrentTags: append([]string{}, item.Task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if item := m.getTaskAtCursor(); item != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", item.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
		}
		return m, nil

	// Score editor
	case "S":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewAllTasks,
				ViewName:   "All Tasks",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *AllTasksViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle bulk operations in multi-select mode
	if m.multiSelectMode && m.selectedCount() > 0 {
		switch prefix {
		case ",":
			switch key {
			case "s":
				return m.handleBulkStatusChange(models.StatusInProgress)
			case "d":
				return m.handleBulkStatusChange(models.StatusDone)
			case "b":
				return m.handleBulkStatusChange(models.StatusBlocked)
			case "x":
				return m.handleBulkStatusChange(models.StatusCanceled)
			case "o":
				return m.handleBulkStatusChange(models.StatusTodo)
			case "r":
				return m.handleBulkStatusChange(models.StatusReview)
			}
		case "!":
			switch key {
			case "1", "2", "3", "4", "5":
				priority := int(key[0] - '0')
				return m.handleBulkPriorityChange(priority)
			}
		case "d":
			switch key {
			case "t":
				return m.handleBulkDueDateChange(0)
			case "m":
				return m.handleBulkDueDateChange(1)
			case "w":
				return m.handleBulkDueDateChange(7)
			case "n":
				return m.handleBulkDueDateChange(30)
			case "0":
				return m.handleBulkClearDueDate()
			}
		case "m":
			section, ok := sectionForMoveKey(key)
			if ok {
				return m.handleBulkSectionMove(section)
			}
		}
		return m, nil
	}

	switch prefix {
	case "z":
		switch key {
		case "R":
			// Expand all groups
			m.collapsedGroups = make(map[string]bool)
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		case "M":
			// Collapse all groups
			for _, group := range m.groups {
				m.collapsedGroups[group.Name] = true
			}
			m.calculateNavigation()
			// Snap cursor to nearest group header
			if len(m.groupStarts) > 0 {
				best := m.groupStarts[0]
				for _, start := range m.groupStarts {
					if start <= m.cursor {
						best = start
					}
				}
				m.cursor = best
			}
			m.clampCursor()
			return m, nil
		}
		return m, nil

	case "!":
		switch key {
		case "1", "2", "3", "4", "5":
			priority := int(key[0] - '0')
			return m.changePriority(priority)
		}
		return m, nil

	case "F":
		return m.handleAllTasksFilterPrefix(key)
	case "Ft":
		return m.handleAllTasksFilterTypePrefix(key)
	case "Fs":
		return m.handleAllTasksFilterStatusPrefix(key)
	case "Fa":
		return m.handleAllTasksFilterAttrPrefix(key)
	case "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap":
		return m.handleAllTasksFilterAttrSubPrefix(prefix, key)
	case "FS":
		return m.handleAllTasksFilterScorePrefix(key)

	case ",":
		// Status operations
		switch key {
		case "s":
			return m.changeTaskStatus(models.StatusInProgress)
		case "d":
			return m.changeTaskStatus(models.StatusDone)
		case "b":
			return m.changeTaskStatus(models.StatusBlocked)
		case "x":
			return m.changeTaskStatus(models.StatusCanceled)
		case "o":
			return m.changeTaskStatus(models.StatusTodo)
		case "r":
			return m.changeTaskStatus(models.StatusReview)
		}

	case "d":
		// Due date operations
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
			// Open date picker
			if item := m.getTaskAtCursor(); item != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: item.Task, Project: item.Project}
				}
			}
		}

	case "g":
		switch key {
		case "g":
			// gg: Go to first item
			m.cursor = 0
			return m, nil
		case "p":
			// gp: Go to project (navigate to task in its project tree)
			if item := m.getTaskAtCursor(); item != nil {
				return m, func() tea.Msg {
					return AllTasksSelectMsg{Task: item.Task, Project: item.Project}
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

	case "c":
		// Create linked task or navigate
		item := m.getTaskAtCursor()
		if item == nil {
			return m, nil
		}
		task := item.Task
		project := item.Project

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

// changeTaskStatus changes the status of the task at cursor.
func (m *AllTasksViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyStatusChangePS(m.projectService, item.Project, item.Task.ID, newStatus)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// changePriority changes the priority of the task at cursor.
func (m *AllTasksViewModel) changePriority(priority int) (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyPriorityChangePS(m.projectService, item.Project, item.Task.ID, priority)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *AllTasksViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyQuickDueDatePS(m.projectService, item.Project, item.Task.ID, daysFromNow)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// clearDueDate clears the due date from the task at cursor.
func (m *AllTasksViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyClearDueDatePS(m.projectService, item.Project, item.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// toggleActive toggles the Active flag on the task at cursor.
func (m *AllTasksViewModel) toggleActive() (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyToggleActivePS(m.projectService, item.Project, item.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// sectionMove moves the task at cursor to the specified GTD section.
func (m *AllTasksViewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applySectionChangePS(m.projectService, item.Project, item.Task.ID, section)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// deleteTask deletes the task at cursor.
func (m *AllTasksViewModel) deleteTask() (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}

	result := applyDeleteTaskPS(m.projectService, item.Project, item.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadAllTasks()
	m.clampCursor()
	return m, result.Cmd
}

// clampCursor ensures cursor is within valid range.
func (m *AllTasksViewModel) clampCursor() {
	total := m.totalNavigableItems()
	if m.cursor >= total {
		m.cursor = total - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *AllTasksViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false for the all tasks view.
func (m *AllTasksViewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode, filters, multi-select, or confirm dialog are active.
func (m *AllTasksViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasActiveFilters() || m.multiSelectMode || m.confirm.IsActive()
}

// GetHelpItems returns help items for this view.
func (m *AllTasksViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewAllTasks)
}

// HasActiveFilters returns true if any filter is currently active.
func (m *AllTasksViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter) || len(m.projectFilter) > 0
}

// toggleProjectFilter toggles the project of the task under cursor in/out of the filter set.
func (m *AllTasksViewModel) toggleProjectFilter() {
	item := m.getTaskAtCursor()
	if item == nil {
		return
	}
	projectID := item.Project.ID
	if m.projectFilter[projectID] {
		delete(m.projectFilter, projectID)
	} else {
		m.projectFilter[projectID] = true
	}
	m.applyFiltersAndGroup()
	m.clampCursor()
}

// ClearAllFilters resets all filters to their default state.
func (m *AllTasksViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
	m.projectFilter = make(map[string]bool)
}

// ShowAllTasksMsg is sent to open the all tasks view.
type ShowAllTasksMsg struct{}

// CloseAllTasksMsg is sent when the all tasks view should close.
type CloseAllTasksMsg struct{}

// AllTasksSelectMsg is sent when a task is selected for navigation.
type AllTasksSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// AllTasksRefreshMsg is sent to refresh the all tasks view.
type AllTasksRefreshMsg struct{}
