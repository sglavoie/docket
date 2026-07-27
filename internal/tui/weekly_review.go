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

// ShowWeeklyReviewMsg opens the weekly review view over the captured project
// set. An omitted set retains the legacy app-wide entry-point behavior.
type ShowWeeklyReviewMsg struct {
	ProjectSet ProjectSetScope
}

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
	PrimaryViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[TaskIdentity]bool
	dailyPlanIDs map[TaskIdentity]bool
	projectSet   ProjectSetScope

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
func NewWeeklyReviewModel(runtime *config.RuntimeState, mgr *config.Manager) *WeeklyReviewModel {
	return NewWeeklyReviewModelForProjectSet(runtime, mgr, ProjectSetScope{})
}

// NewWeeklyReviewModelForProjectSet creates a review over the exact project set
// captured by the launching Outliner scope.
func NewWeeklyReviewModelForProjectSet(
	runtime *config.RuntimeState,
	mgr *config.Manager,
	projectSet ProjectSetScope,
) *WeeklyReviewModel {
	m := &WeeklyReviewModel{
		PrimaryViewBundle: NewPrimaryViewBundle(mgr, runtime),
		projectSet:        canonicalProjectSetScope(projectSet),
		collapsedSections: make(map[int]bool),
		reviewedSections:  make(map[WeeklyReviewSection]bool),
		mutationCounts:    make(map[string]int),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		tabs:              TabState{Names: weeklyReviewTabNames},
		FilterMixin:       FilterMixin{filter: newFilterState()},
	}
	if t := runtime.Planning().GetLastWeeklyReviewAt(); t != nil {
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
	// Uppercase-X deletion now routes through the shell-owned canonical
	// TaskDeleteRequestedMsg confirmation, so the only view-local modal remaining
	// is the weekly-review summary overlay.
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
	}
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

// ShouldCaptureEsc reports whether Weekly Review owns an internal dismissal layer.
func (m *WeeklyReviewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps()) || m.showingSummary
}

// GetHelpItems returns help items for this view.
func (m *WeeklyReviewModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		getGlobalHelpItems(),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
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
	m.ToggleTaskIdentity(taskIdentityFor(wt.Task, wt.Project))
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *WeeklyReviewModel) selectAllVisible() {
	for _, tasks := range [][]WeeklyReviewTask{
		m.staleTasks, m.priorityMismatch, m.upcomingWeek, m.recurringOverview,
	} {
		for _, t := range tasks {
			m.SelectTaskIdentity(taskIdentityFor(t.Task, t.Project))
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *WeeklyReviewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as TaskProjectRef slice.
func (m *WeeklyReviewModel) getSelectedBulkItems() []TaskProjectRef {
	var items []TaskProjectRef
	for _, tasks := range [][]WeeklyReviewTask{
		m.staleTasks, m.priorityMismatch, m.upcomingWeek, m.recurringOverview,
	} {
		for _, t := range tasks {
			if m.IsTaskIdentitySelected(taskIdentityFor(t.Task, t.Project)) {
				items = append(items, TaskProjectRef(t))
			}
		}
	}
	return items
}
