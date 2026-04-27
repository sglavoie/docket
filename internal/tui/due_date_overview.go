package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
	dueDateTabOverdue  = 1
	dueDateTabToday    = 2
	dueDateTabThisWeek = 3
	dueDateTabLater    = 4
)

// DueDateOverviewModel displays all tasks with due dates across all projects.
type DueDateOverviewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	projectService *ProjectService

	// Grouped tasks
	overdue  []CrossProjectTask
	today    []CrossProjectTask
	thisWeek []CrossProjectTask
	later    []CrossProjectTask

	// Tabs
	tabs TabState

	// Filter
	FilterMixin

	// Navigation
	totalTasks  int
	groupStarts []int // Starting index of each group in flattened view

	// Inline preview
	previewMode bool
}

// NewDueDateOverviewModel creates a new due date overview model.
func NewDueDateOverviewModel(cfg *config.Config, mgr *config.Manager) *DueDateOverviewModel {
	m := &DueDateOverviewModel{
		ViewBundle:     ViewBundle{config: cfg, configManager: mgr},
		projectService: NewProjectService(),
		FilterMixin:    FilterMixin{filter: newFilterState()},
		tabs:           TabState{Names: dueDateTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
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

			dt := CrossProjectTask{Task: task, Project: project}
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
	sortByDueDate := func(tasks []CrossProjectTask) {
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
	switch m.tabs.Active {
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
func (m *DueDateOverviewModel) visibleGroups() (overdue, today, thisWeek, later []CrossProjectTask) {
	switch m.tabs.Active {
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
func (m *DueDateOverviewModel) getTaskAtCursor() *CrossProjectTask {
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
		m.setStatus,
	); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case DueDateOverviewRefreshMsg:
		m.loadTasksWithDueDates()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *DueDateOverviewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.ClearError()

	// Handle confirm dialog
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return m.deleteTask()
		}
		return m, nil
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmct.", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Tab navigation
	if handleTabCycleKey(key, &m.tabs, true, func() {
		m.applyTabFilter()
	}) {
		return m, nil
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
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

	// Toggle active flag
	case "*":
		return m.toggleActive()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)
	}

	// Shared task actions: e, r, y/Y/ctrl+y, t, Z, D, ?
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	if handled, cmd := handleListNavigation(key, &m.cursor, m.totalTasks, m.Height(), &m.cursorFlash); handled {
		return m, cmd
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

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		if pfx == "z" && k == "p" {
			m.previewMode = !m.previewMode
			return m, nil, true
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// prefixDispatchView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return GlobalSearchSelectMsg{Task: task, Project: project}
}

func (m *DueDateOverviewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleDueDateFilterPrefix(key)
}

func (m *DueDateOverviewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleDueDateFilterSubPrefix(prefix, key)
}

// commonActionView implementation for DueDateOverviewModel.

func (m *DueDateOverviewModel) viewID() ViewType { return ViewDueDateOverview }
func (m *DueDateOverviewModel) viewName() string { return "Due Date Overview" }

func (m *DueDateOverviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DueDateOverviewModel) getPersister() Persister           { return m.projectService }
func (m *DueDateOverviewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *DueDateOverviewModel) reloadData()                       { m.loadTasksWithDueDates() }
func (m *DueDateOverviewModel) setErr(err error)                  { m.SetError(err) }
func (m *DueDateOverviewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

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
	m.ClampCursorTo(m.totalTasks)
}

// escSteps defines the Esc dismissal cascade: filters.
func (m *DueDateOverviewModel) escSteps() []EscStep {
	return []EscStep{
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.loadTasksWithDueDates()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, or confirm dialog are active.
func (m *DueDateOverviewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}

// GetHelpItems returns help items for this view.
func (m *DueDateOverviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDueDateOverview)
}

func (m *DueDateOverviewModel) HandledKeys() []string {
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
			"g", "gp",
			"enter", "F", "m", "t",
			"1", "2", "3", "4", "5",
			"esc",
		},
	)
}

// CloseDueDateOverviewMsg is sent when the overview should close.
type CloseDueDateOverviewMsg struct{ PopViewMsg }

// DueDateOverviewSelectMsg is sent when a task is selected.
type DueDateOverviewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowDueDateOverviewMsg is sent to open the due date overview.
type ShowDueDateOverviewMsg struct{}

// DueDateOverviewRefreshMsg is sent to refresh the due date overview.
type DueDateOverviewRefreshMsg struct{}

func (m *DueDateOverviewModel) StatusBarLabel() string { return "Due Dates" }

// IsFullHeight returns true because the due date overview renders its own footer.
func (m *DueDateOverviewModel) IsFullHeight() bool { return true }
