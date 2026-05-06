package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
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

	// Multi-select
	MultiSelectState

	// Navigation
	totalTasks  int
	groupStarts []int // Starting index of each group in flattened view

	// Inline preview
	previewMode bool
}

// NewDueDateOverviewModel creates a new due date overview model.
func NewDueDateOverviewModel(cfg *config.Config, mgr *config.Manager) *DueDateOverviewModel {
	m := &DueDateOverviewModel{
		ViewBundle:       ViewBundle{config: cfg, configManager: mgr},
		projectService:   NewProjectService(),
		FilterMixin:      FilterMixin{filter: newFilterState()},
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:             TabState{Names: dueDateTabNames},
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						model, cmd := m.deleteTask()
						return model, cmd, true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
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
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
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

// escSteps defines the Esc dismissal cascade: multi-select → filters.
func (m *DueDateOverviewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
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
	km := m.buildKeyMap()
	out := km.HelpItems()
	result := make([]HelpItem, len(out))
	for i, h := range out {
		result[i] = HelpItem{Key: h.Key, Description: h.Description, Category: h.Category}
	}
	return result
}

func (m *DueDateOverviewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
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

// Multi-select helpers.

func (m *DueDateOverviewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *DueDateOverviewModel) toggleTaskSelection() {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return
	}
	m.ToggleAndUpdateStatus(dt.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *DueDateOverviewModel) selectAllVisible() {
	overdue, today, thisWeek, later := m.visibleGroups()
	for _, group := range [][]CrossProjectTask{overdue, today, thisWeek, later} {
		for _, t := range group {
			m.selectedTasks[t.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *DueDateOverviewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *DueDateOverviewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, group := range [][]CrossProjectTask{m.overdue, m.today, m.thisWeek, m.later} {
		for _, t := range group {
			if m.selectedTasks[t.Task.ID] {
				items = append(items, bulkTaskItem{Task: t.Task, Project: t.Project})
			}
		}
	}
	return items
}
