package tui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
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
	InteractiveViewState

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
	MultiSelectState

	// Navigation
	scrollOffset int
	groupStarts  []int // cursor positions of group headers

	// UI feedback
	err error

	// Dimensions
	width  int
	height int
}

// NewAllTasksViewModel creates a new all tasks view model.
func NewAllTasksViewModel(cfg *config.Config) *AllTasksViewModel {
	m := &AllTasksViewModel{
		config:           cfg,
		projectService:   NewProjectService(),
		projectCache:     make(map[string]*models.Project),
		collapsedGroups:  make(map[string]bool),
		projectFilter:    make(map[string]bool),
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		sortField:        SortByDefault,
		sortAscending:    true,
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
	today := util.StartOfDay(now)

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
			dueDay := util.StartOfDay(*item.Task.DueDate)
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
				return secondaryDeleteTask(m)
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
		return secondaryToggleActive(m)

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		if field, isReset, ok := SortFieldForKey(key); ok {
			if isReset {
				m.sortField = SortByDefault
				m.sortAscending = true
				m.applyFiltersAndGroup()
				m.clampCursor()
				m.statusMessage = "Sort reset"
			} else {
				m.cycleSortField(field)
			}
		}
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
	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardKey(key,
			func() *models.Task {
				if item := m.getTaskAtCursor(); item != nil {
					return item.Task
				}
				return nil
			},
			nil, nil,
		); handled {
			return m, cmd
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
			if status, ok := StatusForPrefixKey(key); ok {
				return m.handleBulkStatusChange(status)
			}
		case "!":
			if priority, ok := PriorityForPrefixKey(key); ok {
				return m.handleBulkPriorityChange(priority)
			}
		case "d":
			if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
				if isClear {
					return m.handleBulkClearDueDate()
				}
				return m.handleBulkDueDateChange(days)
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
		if _, ok := PriorityForPrefixKey(key); ok {
			return secondaryChangePriority(m, key)
		}

	case "F":
		return m.handleAllTasksFilterPrefix(key)
	case "Ft", "Fs", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleAllTasksFilterSubPrefix(prefix, key)

	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			return secondaryChangeStatus(m, status)
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if isClear {
				return secondaryClearDueDate(m)
			}
			return secondarySetQuickDueDate(m, days)
		}
		if key == "d" {
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
		return secondarySectionMove(m, section)

	case "c":
		item := m.getTaskAtCursor()
		if item == nil {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, item.Task, item.Project, false, false); handled {
			return m, cmd
		}
		return m, nil
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// secondaryOpsView implementation for AllTasksViewModel.

func (m *AllTasksViewModel) getSecondaryTaskRef() (*models.Task, *models.Project) {
	item := m.getTaskAtCursor()
	if item == nil {
		return nil, nil
	}
	return item.Task, item.Project
}

func (m *AllTasksViewModel) getSecondaryPersister() Persister { return m.projectService }
func (m *AllTasksViewModel) reloadSecondaryData()             { m.loadAllTasks() }
func (m *AllTasksViewModel) clampSecondaryCursor()            { m.clampCursor() }
func (m *AllTasksViewModel) setSecondaryErr(err error)        { m.err = err }
func (m *AllTasksViewModel) setSecondaryStatus(msg string)    { m.statusMessage = msg }

// clampCursor ensures cursor is within valid range.
func (m *AllTasksViewModel) clampCursor() {
	m.cursor = ClampCursor(m.cursor, m.totalNavigableItems())
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
