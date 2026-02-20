package tui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
	BaseView
	InteractiveViewState

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
	totalTasks  int
	groupStarts []int // Starting index of each group in flattened view
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
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*task.DueDate)

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

func (m *DueDateOverviewModel) Init() tea.Cmd {
	return nil
}

func (m *DueDateOverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(
		msg,
		m.SetSize,
		m.loadTasksWithDueDates,
		m.clampCursor,
		func(s string) { m.statusMessage = s },
	); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
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
	m.ClearError()

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

	// Activate prefix mode
	switch key {
	case ",", "d", "g":
		m.pendingPrefix = key
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	case "m":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "m"
		}
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

	// Toggle active flag
	case "*":
		return m.toggleActive()

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
	case "y", "Y", "ctrl+y":
		var task *models.Task
		if dt := m.getTaskAtCursor(); dt != nil {
			task = dt.Task
		}
		if cmd, handled := clipboardCmd(key, task); handled {
			return m, cmd
		}
		return m, nil

	// Tags
	case "t":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: dt.Task, Project: dt.Project, CurrentTags: append([]string{}, dt.Task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: dt.Task, Project: dt.Project}
			}
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if dt := m.getTaskAtCursor(); dt != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", dt.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
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

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, nil); handled {
		return model, cmd
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// prefixDispatchView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *DueDateOverviewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return GlobalSearchSelectMsg{Task: task, Project: project}
}

func (m *DueDateOverviewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleDueDateFilterPrefix(key)
}

func (m *DueDateOverviewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleDueDateFilterSubPrefix(prefix, key)
}

// taskOpsView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DueDateOverviewModel) getPersister() Persister { return m.projectService }
func (m *DueDateOverviewModel) getDataDir() string      { return m.config.DataDir }
func (m *DueDateOverviewModel) reloadData()             { m.loadTasksWithDueDates() }
func (m *DueDateOverviewModel) setErr(err error)        { m.SetError(err) }
func (m *DueDateOverviewModel) setStatus(msg string)    { m.statusMessage = msg }

func (m *DueDateOverviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, key)
}

func (m *DueDateOverviewModel) toggleActive() (tea.Model, tea.Cmd) {
	return taskOpsToggleActive(m)
}

func (m *DueDateOverviewModel) deleteTask() (tea.Model, tea.Cmd) {
	return taskOpsDeleteTask(m)
}

// clampCursor ensures cursor is within valid range.
func (m *DueDateOverviewModel) clampCursor() {
	m.cursor = ClampCursor(m.cursor, m.totalTasks)
}

// HasTextInput returns false for the due date overview.
func (m *DueDateOverviewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode, filters, or confirm dialog are active.
func (m *DueDateOverviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasActiveFilters() || m.confirm.IsActive()
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
