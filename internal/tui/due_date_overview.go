package tui

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// DueDateTask wraps a task with its parent project for cross-project display.
type DueDateTask struct {
	Task    *models.Task
	Project *models.Project
}

// DueDateGroup represents a category of tasks by due date urgency.
type DueDateGroup int

const (
	GroupOverdue DueDateGroup = iota
	GroupToday
	GroupThisWeek
	GroupLater
)

var dueDateTabNames = []string{"All", "Overdue", "Today", "This Week", "Later"}

const (
	dueDateTabAll      = 0
	dueDateTabOverdue  = 1
	dueDateTabToday    = 2
	dueDateTabThisWeek = 3
	dueDateTabLater    = 4
)

// DueDateOverviewModel displays all tasks with due dates across all projects.
type DueDateOverviewModel struct {
	config         *config.Config
	projectService *ProjectService

	// Grouped tasks
	overdue  []DueDateTask
	today    []DueDateTask
	thisWeek []DueDateTask
	later    []DueDateTask

	// Tabs
	activeTab int

	// Filter
	filter CommonFilterState

	// Navigation
	cursor      int // Absolute position across all groups
	totalTasks  int
	groupStarts []int // Starting index of each group in flattened view

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "," or "d" when waiting for second key

	// UI feedback
	statusMessage string
	err           error

	// Dimensions
	width  int
	height int
}

// NewDueDateOverviewModel creates a new due date overview model.
func NewDueDateOverviewModel(cfg *config.Config) *DueDateOverviewModel {
	m := &DueDateOverviewModel{
		config:         cfg,
		projectService: NewProjectService(),
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
	}
	m.loadTasksWithDueDates()
	return m
}

// loadTasksWithDueDates aggregates tasks with due dates from all active projects.
func (m *DueDateOverviewModel) loadTasksWithDueDates() {
	m.overdue = nil
	m.today = nil
	m.thisWeek = nil
	m.later = nil

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if task.DueDate == nil || task.Status.IsComplete() {
				continue
			}

			// Apply common filters (no DueDate filter — the view IS organized by due date)
			if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(task, &m.filter, project.ID, m.config) {
				continue
			}

			dt := DueDateTask{Task: task, Project: project}
			switch m.categorizeTask(task) {
			case GroupOverdue:
				m.overdue = append(m.overdue, dt)
			case GroupToday:
				m.today = append(m.today, dt)
			case GroupThisWeek:
				m.thisWeek = append(m.thisWeek, dt)
			case GroupLater:
				m.later = append(m.later, dt)
			}
		}
	}

	// Sort each group by due date (earliest first)
	sortByDueDate := func(tasks []DueDateTask) {
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Task.DueDate.Before(*tasks[j].Task.DueDate)
		})
	}
	sortByDueDate(m.overdue)
	sortByDueDate(m.today)
	sortByDueDate(m.thisWeek)
	sortByDueDate(m.later)

	// Calculate navigation indices based on active tab
	m.recalcTotalTasks()
}

// applyTabFilter recomputes navigation after a tab change.
func (m *DueDateOverviewModel) applyTabFilter() {
	m.recalcTotalTasks()
	m.clampCursor()
}

// recalcTotalTasks updates totalTasks and groupStarts based on the active tab.
func (m *DueDateOverviewModel) recalcTotalTasks() {
	switch m.activeTab {
	case dueDateTabOverdue:
		m.totalTasks = len(m.overdue)
		m.groupStarts = []int{0}
	case dueDateTabToday:
		m.totalTasks = len(m.today)
		m.groupStarts = []int{0}
	case dueDateTabThisWeek:
		m.totalTasks = len(m.thisWeek)
		m.groupStarts = []int{0}
	case dueDateTabLater:
		m.totalTasks = len(m.later)
		m.groupStarts = []int{0}
	default: // All
		m.totalTasks = len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later)
		m.groupStarts = []int{
			0,
			len(m.overdue),
			len(m.overdue) + len(m.today),
			len(m.overdue) + len(m.today) + len(m.thisWeek),
		}
	}
}

// visibleGroups returns the groups visible under the current tab.
func (m *DueDateOverviewModel) visibleGroups() (overdue, today, thisWeek, later []DueDateTask) {
	switch m.activeTab {
	case dueDateTabOverdue:
		return m.overdue, nil, nil, nil
	case dueDateTabToday:
		return nil, m.today, nil, nil
	case dueDateTabThisWeek:
		return nil, nil, m.thisWeek, nil
	case dueDateTabLater:
		return nil, nil, nil, m.later
	default:
		return m.overdue, m.today, m.thisWeek, m.later
	}
}

// categorizeTask determines which urgency group a task belongs to.
func (m *DueDateOverviewModel) categorizeTask(task *models.Task) DueDateGroup {
	if task.DueDate == nil {
		return GroupLater
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		return GroupOverdue
	}
	if dueDay.Equal(today) {
		return GroupToday
	}
	weekEnd := today.AddDate(0, 0, 7)
	if dueDay.Before(weekEnd) {
		return GroupThisWeek
	}
	return GroupLater
}

// getTaskAtCursor returns the task and project at the current cursor position.
func (m *DueDateOverviewModel) getTaskAtCursor() *DueDateTask {
	if m.totalTasks == 0 {
		return nil
	}

	overdue, today, thisWeek, later := m.visibleGroups()

	cursor := m.cursor
	if cursor < len(overdue) {
		return &overdue[cursor]
	}
	cursor -= len(overdue)
	if cursor < len(today) {
		return &today[cursor]
	}
	cursor -= len(today)
	if cursor < len(thisWeek) {
		return &thisWeek[cursor]
	}
	cursor -= len(thisWeek)
	if cursor < len(later) {
		return &later[cursor]
	}
	return nil
}

// HasActiveSearch returns false (no search in this view).
func (m *DueDateOverviewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *DueDateOverviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DueDateOverviewModel) Init() tea.Cmd {
	return nil
}

func (m *DueDateOverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case DueDateOverviewRefreshMsg:
		m.loadTasksWithDueDates()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *DueDateOverviewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Activate prefix mode
	switch key {
	case ",", "d", "g":
		m.pendingPrefix = key
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	}

	// Tab navigation
	switch key {
	case "[", "h":
		if m.activeTab > 0 {
			m.activeTab--
		} else {
			m.activeTab = len(dueDateTabNames) - 1
		}
		m.applyTabFilter()
		return m, nil
	case "]", "l":
		if m.activeTab < len(dueDateTabNames)-1 {
			m.activeTab++
		} else {
			m.activeTab = 0
		}
		m.applyTabFilter()
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.loadTasksWithDueDates()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseDueDateOverviewMsg{}
		}

	case "enter":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return DueDateOverviewSelectMsg{Task: dt.Task, Project: dt.Project}
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

	// Priority changes
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)

	// Edit task
	case "e":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: dt.Task}
			}
		}
		return m, nil

	// Refresh
	case "r":
		m.loadTasksWithDueDates()
		m.clampCursor()
		m.statusMessage = "Refreshed"
		return m, nil

	// Copy
	case "y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: dt.Task}
			}
		}
		return m, nil

	case "Y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: dt.Task}
			}
		}
		return m, nil

	case "ctrl+y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: dt.Task}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewDueDateOverview,
				ViewName:   "Due Date Overview",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *DueDateOverviewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case "F":
		return m.handleDueDateFilterPrefix(key)
	case "Ft":
		return m.handleDueDateFilterTypePrefix(key)
	case "Fs":
		return m.handleDueDateFilterStatusPrefix(key)
	case "Fa":
		return m.handleDueDateFilterAttrPrefix(key)
	case "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap":
		return m.handleDueDateFilterAttrSubPrefix(prefix, key)
	case "FS":
		return m.handleDueDateFilterScorePrefix(key)

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
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: dt.Task, Project: dt.Project}
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
			// gp: Go to project (navigate to task in its project)
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return GlobalSearchSelectMsg{Task: dt.Task, Project: dt.Project}
				}
			}
			return m, nil
		}
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// changeTaskStatus changes the status of the task at cursor.
func (m *DueDateOverviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	// Validate status transition
	switch newStatus {
	case models.StatusInProgress:
		if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
			m.statusMessage = "Task must be todo or blocked to start"
			return m, nil
		}
	case models.StatusDone:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
		// Check for incomplete dependencies
		if project.HasIncompleteDependencies(task) {
			m.err = fmt.Errorf("cannot complete: has incomplete dependencies")
			return m, nil
		}
	case models.StatusBlocked:
		if !task.Status.IsActive() || task.Status == models.StatusBlocked {
			m.statusMessage = "Task must be active and not already blocked"
			return m, nil
		}
	case models.StatusCanceled:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			m.statusMessage = "Task is already todo"
			return m, nil
		}
	case models.StatusReview:
		if !task.Status.IsActive() || task.Status == models.StatusReview {
			m.statusMessage = "Task must be active and not already in review"
			return m, nil
		}
	}

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task status
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Status = newStatus
		if newStatus.IsComplete() {
			now := time.Now()
			t.CompletedAt = &now
		} else {
			t.CompletedAt = nil
		}
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
	m.loadTasksWithDueDates()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Task #%d → %s", task.ShortID, newStatus)

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *DueDateOverviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	task := dt.Task
	project := dt.Project

	if task.Priority == priority {
		return m, nil
	}

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task priority
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
	m.loadTasksWithDueDates()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m, nil
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *DueDateOverviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Set due date
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = &newDate
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
	m.loadTasksWithDueDates()
	m.clampCursor()

	label := "today"
	switch daysFromNow {
	case 1:
		label = "tomorrow"
	case 7:
		label = "next week"
	case 30:
		label = "next month"
	}
	m.statusMessage = fmt.Sprintf("Due date set to %s", label)

	return m, func() tea.Msg {
		return DueDateChangedMsg{Task: task}
	}
}

// clearDueDate clears the due date from the task at cursor.
func (m *DueDateOverviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	if task.DueDate == nil {
		m.statusMessage = "Task has no due date"
		return m, nil
	}

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Clear due date
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = nil
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
	m.loadTasksWithDueDates()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, func() tea.Msg {
		return DueDateChangedMsg{Task: task}
	}
}

// clampCursor ensures cursor is within valid range.
func (m *DueDateOverviewModel) clampCursor() {
	if m.cursor >= m.totalTasks {
		m.cursor = m.totalTasks - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *DueDateOverviewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false for the due date overview.
func (m *DueDateOverviewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode or filters are active.
func (m *DueDateOverviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasActiveFilters()
}

// GetHelpItems returns help items for this view.
func (m *DueDateOverviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDueDateOverview)
}

// CloseDueDateOverviewMsg is sent when the overview should close.
type CloseDueDateOverviewMsg struct{}

// DueDateOverviewSelectMsg is sent when a task is selected.
type DueDateOverviewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowDueDateOverviewMsg is sent to open the due date overview.
type ShowDueDateOverviewMsg struct{}

// DueDateOverviewRefreshMsg is sent to refresh the due date overview.
type DueDateOverviewRefreshMsg struct{}
