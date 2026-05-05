package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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
	ViewBundle

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
	tabs              TabState             // 0=Due, 1=Active, 2=In Review, 3=Stale, 4=Done
	scrollOffset      int
	previewMode       bool
}

// NewDailyReviewModel creates a new daily review model.
func NewDailyReviewModel(cfg *config.Config, mgr *config.Manager) *DailyReviewModel {
	m := &DailyReviewModel{
		ViewBundle:        ViewBundle{config: cfg, configManager: mgr},
		collapsedSections: make(map[int]bool),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		tabs:              TabState{Names: dailyReviewTabNames},
	}
	if t := cfg.GetLastDailyReviewAt(); t != nil && isToday(*t) {
		m.reviewCompleted = true
		m.completedAt = *t
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
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
	key := msg.String()

	// Handle confirm dialog first
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return taskOpsDeleteTask(m)
		}
		return m, nil
	}

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Tab switching
	if handleTabCycleKey(key, &m.tabs, func() {
		m.focusedSection = nil
		m.scrollOffset = 0
		m.calculateNavigation()
		m.clampCursor()
	}) {
		return m, nil
	}

	// Multi-select key handling
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case "space":
			m.toggleTaskSelection()
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAll()
			return m, nil
		}
		switch key {
		case ",", "!", "d", "s", "m", ".":
			m.pendingPrefix = key
			return m, nil
		}
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmctb.", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, m.totalTasks, m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseDailyReviewMsg{}
		}

	case "enter":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return DailyReviewSelectMsg{Task: dt.Task, Project: dt.Project}
			}
		}
		return m, nil

	// Section navigation (reset focus to show all sections)
	case "n":
		m.focusedSection = nil
		m.calculateNavigation()
		return m.goToNextSection()
	case "N":
		m.focusedSection = nil
		m.calculateNavigation()
		return m.goToPrevSection()

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return taskOpsChangePriority(m, key)

	// Toggle daily review completion
	case "R":
		if m.reviewCompleted {
			m.reviewCompleted = false
			return m, func() tea.Msg { return ResetDailyReviewMsg{} }
		}
		m.reviewCompleted = true
		m.completedAt = time.Now()
		return m, func() tea.Msg { return MarkDailyReviewDoneMsg{} }
	}

	// Common task actions (tags, snooze, delete, edit, refresh, copy, help)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *DailyReviewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil
	}

	if m.multiSelectMode && m.SelectedCount() > 0 {
		if model, cmd, handled := dispatchBulkPrefix(prefix, key, m, m.getSelectedBulkItems()); handled {
			m.ExitMultiSelect()
			return model, cmd
		}
		return m, nil
	}

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		switch pfx {
		case "z":
			if k == "p" {
				m.previewMode = !m.previewMode
				return m, nil, true
			}
			handleZPrefix(k, func() {
				ExpandAllSections(m.collapsedSections)
				m.calculateNavigation()
				m.clampCursor()
			}, func() {
				sectionInts := make([]int, len(m.sectionOrder))
				for i, s := range m.sectionOrder {
					sectionInts[i] = int(s)
				}
				CollapseAllSections(m.collapsedSections, sectionInts)
				m.calculateNavigation()
				m.clampCursor()
			})
			return m, nil, true

		case "g":
			// Let gg (jump-to-top) and gp (go-to-project) fall through to common dispatch
			if k == "g" || k == "p" {
				return nil, nil, false
			}
			var targetSection *DailyReviewSection
			switch k {
			case "d":
				s := SectionOverdue
				targetSection = &s
			case "t":
				s := SectionDueToday
				targetSection = &s
			case "s":
				s := SectionDueSoon
				targetSection = &s
			case "l":
				s := SectionDueLater
				targetSection = &s
			case "f":
				s := SectionFuture
				targetSection = &s
			case "r":
				s := SectionInReview
				targetSection = &s
			case "S":
				s := SectionOnStandby
				targetSection = &s
			case "o":
				s := SectionOld
				targetSection = &s
			}
			if targetSection != nil {
				model, cmd := m.toggleSectionFocus(*targetSection)
				return model, cmd, true
			}
			return m, nil, true
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
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
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}

func (m *DailyReviewModel) HandledKeys() []string {
	return concatKeys(
		listNavigationKeys(),
		statusPrefixKeys(),
		priorityPrefixKeys(),
		dueDatePrefixKeys(),
		snoozePrefixKeys(),
		linkedTaskPrefixKeys(),
		stagePrefixKeys(),
		blockerPrefixKeySet(),
		commonActionKeys(),
		[]string{
			"g",
			"shift+left", "shift+right",
			"zR", "zM", "zp",
			"tab", "n", "N",
			"gd", "gt", "gs", "gl", "gf", "gr", "gS", "go", "gp",
			"enter", "F", "m", "t", "R",
			"1", "2", "3", "4", "5",
			"v", "esc",
		},
	)
}

// isToday returns true if t falls on the current calendar day.
func isToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.YearDay() == now.YearDay()
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
