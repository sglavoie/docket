package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// WeeklyReviewProject wraps a project with computed metrics.
type WeeklyReviewProject struct {
	Project          *models.Project
	InboxCount       int
	BlockedCount     int
	InProgressCount  int
	NoActivityDays   int
	OverdueCount     int
	HighPriorityTodo int
}

// WeeklyReviewTask wraps a task with its parent project.
type WeeklyReviewTask struct {
	Task    *models.Task
	Project *models.Project
}

// WeeklyReviewSection represents the different sections in weekly review.
type WeeklyReviewSection int

const (
	WeeklySectionStaleProjects WeeklyReviewSection = iota
	WeeklySectionInboxBacklog
	WeeklySectionBlockedWork
	WeeklySectionOvercommitted
	WeeklySectionStaleTasks
	WeeklySectionPriorityMismatch
	WeeklySectionUpcomingWeek
	WeeklySectionRecurringOverview
)

// ShowWeeklyReviewMsg opens the weekly review view.
type ShowWeeklyReviewMsg struct{}

// CloseWeeklyReviewMsg closes the weekly review view.
type CloseWeeklyReviewMsg struct{ PopViewMsg }

// WeeklyReviewRefreshMsg requests a refresh of the weekly review data.
type WeeklyReviewRefreshMsg struct{}

// WeeklyReviewSelectProjectMsg navigates to a project from weekly review.
type WeeklyReviewSelectProjectMsg struct {
	Project *models.Project
}

// WeeklyReviewSelectTaskMsg navigates to a task from weekly review.
type WeeklyReviewSelectTaskMsg struct {
	Task    *models.Task
	Project *models.Project
}

// WeeklyReviewModel displays aggregated project and task information for weekly review.
type WeeklyReviewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	// Project sections
	staleProjects []WeeklyReviewProject
	inboxBacklog  []WeeklyReviewProject
	blockedWork   []WeeklyReviewProject
	overcommitted []WeeklyReviewProject

	// Task sections
	staleTasks        []WeeklyReviewTask
	priorityMismatch  []WeeklyReviewTask
	upcomingWeek      []WeeklyReviewTask
	recurringOverview []WeeklyReviewTask

	// Filter
	FilterMixin

	// Multi-select
	MultiSelectState

	// Navigation
	totalItems        int
	sectionStarts     []int
	collapsedSections map[int]bool
	sectionOrder      []WeeklyReviewSection
	focusedSection    *WeeklyReviewSection
	tabs              TabState // 0=Projects, 1=Tasks, 2=Planning
	scrollOffset      int
	previewMode       bool

	// Guided review flow
	reviewedSections map[WeeklyReviewSection]bool
	mutationCounts   map[string]int
	showingSummary   bool

	// Priority drift detection
	priorityDriftWarning string
	priorityDriftPct     int

	// Review completion state
	reviewCompleted bool
	completedAt     time.Time
}

// NewWeeklyReviewModel creates a new weekly review model.
func NewWeeklyReviewModel(cfg *config.Config, mgr *config.Manager) *WeeklyReviewModel {
	m := &WeeklyReviewModel{
		ViewBundle:        ViewBundle{config: cfg, configManager: mgr},
		collapsedSections: make(map[int]bool),
		reviewedSections:  make(map[WeeklyReviewSection]bool),
		mutationCounts:    make(map[string]int),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:              TabState{Names: weeklyReviewTabNames},
		FilterMixin:       FilterMixin{filter: newFilterState()},
	}
	if t := cfg.GetLastWeeklyReviewAt(); t != nil {
		tYear, tWeek := t.ISOWeek()
		nowYear, nowWeek := time.Now().ISOWeek()
		if tYear == nowYear && tWeek == nowWeek {
			m.reviewCompleted = true
			m.completedAt = *t
			// Mark all sections as reviewed
			allSections := []WeeklyReviewSection{
				WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
				WeeklySectionBlockedWork, WeeklySectionOvercommitted,
				WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
				WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
			}
			for _, s := range allSections {
				m.reviewedSections[s] = true
			}
		}
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.showingSummary },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if key == "esc" {
					return m, func() tea.Msg { return CloseWeeklyReviewMsg{} }, true
				}
				m.showingSummary = false
				return m, nil, true
			},
		},
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						model, cmd := taskOpsDeleteTask(m)
						if m.err == nil {
							m.trackMutation("delete")
						}
						return model, cmd, true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
	m.loadData()
	return m
}

func (m *WeeklyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *WeeklyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadData, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case WeeklyReviewRefreshMsg:
		m.loadData()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *WeeklyReviewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

// handleEnter handles Enter key, navigating to project or task.
func (m *WeeklyReviewModel) handleEnter() (tea.Model, tea.Cmd) {
	if wp := m.getProjectAtCursor(); wp != nil {
		return m, func() tea.Msg {
			return WeeklyReviewSelectProjectMsg{Project: wp.Project}
		}
	}
	if dt := m.getTaskAtCursor(); dt != nil {
		return m, func() tea.Msg {
			return WeeklyReviewSelectTaskMsg{Task: dt.Task, Project: dt.Project}
		}
	}
	return m, nil
}

// HasFocusedSection returns true if viewing a single focused section.
func (m *WeeklyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// escSteps defines the Esc dismissal cascade: section focus.
func (m *WeeklyReviewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
		{Active: m.HasFocusedSection, Clear: func() {
			m.focusedSection = nil
			m.calculateNavigation()
			m.clampCursor()
		}},
	}
}

// ShouldCaptureEsc returns true when prefix mode, section focus, confirm dialog, or summary is active.
func (m *WeeklyReviewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps()) || m.showingSummary
}

func (m *WeeklyReviewModel) viewID() ViewType { return ViewWeeklyReview }
func (m *WeeklyReviewModel) viewName() string { return "Weekly Review" }

// GetHelpItems returns help items for this view.
func (m *WeeklyReviewModel) GetHelpItems() []HelpItem {
	km := m.buildKeyMap()
	out := km.HelpItems()
	result := make([]HelpItem, len(out))
	for i, h := range out {
		result[i] = HelpItem{Key: h.Key, Description: h.Description, Category: h.Category}
	}
	return result
}

func (m *WeeklyReviewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *WeeklyReviewModel) StatusBarLabel() string { return "Weekly Review" }

// IsFullHeight returns true because the weekly review renders its own footer.
func (m *WeeklyReviewModel) IsFullHeight() bool { return true }

// Multi-select helpers.

func (m *WeeklyReviewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *WeeklyReviewModel) toggleTaskSelection() {
	wt := m.getTaskAtCursor()
	if wt == nil {
		return
	}
	m.ToggleAndUpdateStatus(wt.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *WeeklyReviewModel) selectAllVisible() {
	for _, tasks := range [][]WeeklyReviewTask{
		m.staleTasks, m.priorityMismatch, m.upcomingWeek, m.recurringOverview,
	} {
		for _, t := range tasks {
			m.selectedTasks[t.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *WeeklyReviewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *WeeklyReviewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, tasks := range [][]WeeklyReviewTask{
		m.staleTasks, m.priorityMismatch, m.upcomingWeek, m.recurringOverview,
	} {
		for _, t := range tasks {
			if m.selectedTasks[t.Task.ID] {
				items = append(items, bulkTaskItem{Task: t.Task, Project: t.Project})
			}
		}
	}
	return items
}
