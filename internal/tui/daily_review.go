package tui

import (
	tea "github.com/charmbracelet/bubbletea"

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

	// Filter
	FilterMixin

	// Navigation
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections
	focusedSection    *DailyReviewSection  // nil = all shown, otherwise show only this section
	tabs              TabState             // 0=Due, 1=Active, 2=In Review, 3=Stale, 4=Done
	scrollOffset      int
}

// NewDailyReviewModel creates a new daily review model.
func NewDailyReviewModel(cfg *config.Config, mgr *config.Manager) *DailyReviewModel {
	m := &DailyReviewModel{
		ViewBundle:        ViewBundle{config: cfg, configManager: mgr},
		collapsedSections: make(map[int]bool),
		FilterMixin:       FilterMixin{filter: newFilterState()},
		tabs:              TabState{Names: dailyReviewTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.loadTasks()
	return m
}

func (m *DailyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *DailyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, func(s string) { m.statusMessage = s }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case DailyReviewRefreshMsg:
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *DailyReviewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle confirm dialog first
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return taskOpsDeleteTask(m)
		}
		return m, nil
	}

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Tab switching
	if handleTabCycleKey(key, &m.tabs, true, func() {
		m.focusedSection = nil
		m.scrollOffset = 0
		m.calculateNavigation()
		m.clampCursor()
	}) {
		return m, nil
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, "Fz", ",dgsmc", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Navigation
	if handleListNavigation(key, &m.cursor, m.totalTasks, m.Height()) {
		return m, nil
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseDailyReviewMsg{}
		}

	case "q":
		// q clears filters but does not unfocus sections
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.loadTasks()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
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

	// Toggle active flag
	case "*":
		return taskOpsToggleActive(m)

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return taskOpsChangePriority(m, key)
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

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		switch pfx {
		case "z":
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

		case "s":
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
		{Active: m.HasFocusedSection, Clear: func() {
			m.focusedSection = nil
			m.calculateNavigation()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.loadTasks()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
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
