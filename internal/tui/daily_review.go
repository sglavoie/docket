package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// DailyReviewSection represents the different sections in review.
type DailyReviewSection int

const (
	SectionOverdue DailyReviewSection = iota
	SectionDueToday
	SectionDueSoon
	SectionDueLater
	SectionFuture
	SectionInProgress
	SectionBlocked
	SectionInReview
	SectionOnStandby
	SectionOld
	SectionCompleted
)

// DailyReviewModel displays aggregated task information for review.
type DailyReviewModel struct {
	BaseView
	InteractiveViewState
	PrimaryViewBundle

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	// Sections
	overdue    []CrossProjectTask
	dueToday   []CrossProjectTask
	dueSoon    []CrossProjectTask
	dueLater   []CrossProjectTask
	future     []CrossProjectTask
	inProgress []CrossProjectTask
	blocked    []CrossProjectTask
	inReview   []CrossProjectTask
	inStandby  []CrossProjectTask
	old        []CrossProjectTask
	completed  []CrossProjectTask

	// Review completion state
	reviewCompleted bool
	completedAt     time.Time
	reviewedCount   int // number of tasks acted on this session

	// Filter
	FilterMixin

	// Multi-select
	MultiSelectState

	// Navigation
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections
	focusedSection    *DailyReviewSection  // nil = all shown, otherwise show only this section
	tabs              TabState             // 0=Due, 1=Done, 2=Active, 3=In Review, 4=Stale
	scrollOffset      int
	previewMode       bool
}

// NewDailyReviewModel creates a new daily review model.
func NewDailyReviewModel(runtime *config.RuntimeState, mgr *config.Manager) *DailyReviewModel {
	m := &DailyReviewModel{
		PrimaryViewBundle: NewPrimaryViewBundle(mgr, runtime),
		collapsedSections: make(map[int]bool),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:              TabState{Names: dailyReviewTabNames},
	}
	if t := runtime.Planning().GetLastDailyReviewAt(); t != nil && isToday(*t) {
		m.reviewCompleted = true
		m.completedAt = *t
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						// Delete through the coordinator-backed workflow path (folds the
						// durable planning-ref cleanup into the same transaction); the shell
						// reloads Daily Review from the async BatchTransactionResultMsg.
						return m, wfDeleteTask(m, m.getRuntime(), m.getConfigManager()), true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.loadTasks()
	return m
}

func (m *DailyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *DailyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case DailyReviewRefreshMsg:
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *DailyReviewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

// HasFocusedSection returns true if viewing a single focused section.
func (m *DailyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// escSteps defines the Esc dismissal cascade: section focus → filters.
func (m *DailyReviewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
		{Active: m.HasFocusedSection, Clear: func() {
			m.focusedSection = nil
			m.calculateNavigation()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.loadTasks()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, section focus, filters, or confirm dialog are active.
func (m *DailyReviewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}

func (m *DailyReviewModel) viewID() ViewType { return ViewDailyReview }
func (m *DailyReviewModel) viewName() string { return "Review" }

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *DailyReviewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// isToday returns true if t falls on the current calendar day in the local timezone.
func isToday(t time.Time) bool {
	now := time.Now()
	tl := t.In(now.Location())
	return tl.Year() == now.Year() && tl.YearDay() == now.YearDay()
}

func (m *DailyReviewModel) StatusBarLabel() string { return "Daily Review" }

// IsFullHeight returns true because the daily review renders its own footer.
func (m *DailyReviewModel) IsFullHeight() bool { return true }

// Multi-select helpers.

func (m *DailyReviewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *DailyReviewModel) toggleTaskSelection() {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return
	}
	m.ToggleAndUpdateStatus(dt.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *DailyReviewModel) selectAllVisible() {
	for _, tasks := range [][]CrossProjectTask{
		m.overdue, m.dueToday, m.dueSoon, m.dueLater, m.future,
		m.inProgress, m.blocked, m.inReview, m.inStandby, m.old, m.completed,
	} {
		for _, t := range tasks {
			m.selectedTasks[t.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *DailyReviewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *DailyReviewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, tasks := range [][]CrossProjectTask{
		m.overdue, m.dueToday, m.dueSoon, m.dueLater, m.future,
		m.inProgress, m.blocked, m.inReview, m.inStandby, m.old, m.completed,
	} {
		for _, t := range tasks {
			if m.selectedTasks[t.Task.ID] {
				items = append(items, bulkTaskItem{Task: t.Task, Project: t.Project})
			}
		}
	}
	return items
}
