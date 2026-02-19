package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// DailyReviewTask wraps a task with its parent project for cross-project display.
type DailyReviewTask struct {
	Task    *models.Task
	Project *models.Project
}

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

	config *config.Config

	// Sections
	overdue    []DailyReviewTask
	dueToday   []DailyReviewTask
	dueSoon    []DailyReviewTask
	dueLater   []DailyReviewTask
	future     []DailyReviewTask
	inProgress []DailyReviewTask
	blocked    []DailyReviewTask
	inReview   []DailyReviewTask
	inStandby  []DailyReviewTask
	old        []DailyReviewTask
	completed  []DailyReviewTask

	// Filter
	filter CommonFilterState

	// Navigation
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections
	focusedSection    *DailyReviewSection  // nil = all shown, otherwise show only this section
	activeTab         int                  // 0=Due, 1=Active, 2=In Review, 3=Stale, 4=Done
}

// NewDailyReviewModel creates a new daily review model.
func NewDailyReviewModel(cfg *config.Config) *DailyReviewModel {
	m := &DailyReviewModel{
		config:            cfg,
		collapsedSections: make(map[int]bool),
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
	}
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
	if m.confirm.IsActive() {
		confirmed, handled := m.confirm.HandleKey(key)
		if handled {
			if confirmed {
				return taskOpsDeleteTask(m)
			}
			m.statusMessage = ""
			return m, nil
		}
	}

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Tab switching
	switch key {
	case "[", "h":
		CycleTab(&m.activeTab, len(dailyReviewTabNames), -1)
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	case "]", "l":
		CycleTab(&m.activeTab, len(dailyReviewTabNames), 1)
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	}

	// Activate prefix mode
	switch key {
	case ",", "d", "g", "s", "m":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	case "F", "z":
		m.pendingPrefix = key
		return m, nil
	case "c":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "c"
		}
		return m, nil
	}

	// Navigation
	if handleListNavigation(key, &m.cursor, m.totalTasks) {
		return m, nil
	}

	switch key {
	case "esc":
		// If focused on a section, unfocus to show all sections
		if m.focusedSection != nil {
			m.focusedSection = nil
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}
		// If filters are active, clear them before closing
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

	case "q":
		// If filters are active, clear them before closing
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
			switch k {
			case "R":
				ExpandAllSections(m.collapsedSections)
				m.calculateNavigation()
				m.clampCursor()
				return m, nil, true
			case "M":
				sectionInts := make([]int, len(m.sectionOrder))
				for i, s := range m.sectionOrder {
					sectionInts[i] = int(s)
				}
				CollapseAllSections(m.collapsedSections, sectionInts)
				m.calculateNavigation()
				m.clampCursor()
				return m, nil, true
			}
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

// ShouldCaptureEsc returns true when prefix mode, section focus, filters, or confirm dialog are active.
func (m *DailyReviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasFocusedSection() || m.HasActiveFilters() || m.confirm.IsActive()
}

func (m *DailyReviewModel) viewID() ViewType { return ViewDailyReview }
func (m *DailyReviewModel) viewName() string { return "Review" }

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}
