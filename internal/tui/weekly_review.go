package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
)

// ShowWeeklyReviewMsg opens the weekly review view.
type ShowWeeklyReviewMsg struct{}

// CloseWeeklyReviewMsg closes the weekly review view.
type CloseWeeklyReviewMsg struct{}

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

	config *config.Config

	// Project sections
	staleProjects []WeeklyReviewProject
	inboxBacklog  []WeeklyReviewProject
	blockedWork   []WeeklyReviewProject
	overcommitted []WeeklyReviewProject

	// Task sections
	staleTasks       []WeeklyReviewTask
	priorityMismatch []WeeklyReviewTask
	upcomingWeek     []WeeklyReviewTask

	// Navigation
	totalItems        int
	sectionStarts     []int
	collapsedSections map[int]bool
	sectionOrder      []WeeklyReviewSection
	focusedSection    *WeeklyReviewSection
	activeTab         int // 0=Projects, 1=Tasks, 2=Planning
}

// NewWeeklyReviewModel creates a new weekly review model.
func NewWeeklyReviewModel(cfg *config.Config) *WeeklyReviewModel {
	m := &WeeklyReviewModel{
		config:            cfg,
		collapsedSections: make(map[int]bool),
	}
	m.loadData()
	return m
}

func (m *WeeklyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *WeeklyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadData, m.clampCursor, func(s string) { m.statusMessage = s }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case WeeklyReviewRefreshMsg:
		m.loadData()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *WeeklyReviewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle confirmation dialog
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

	// Handle pending prefix keys
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Tab switching
	switch key {
	case "[", "h":
		CycleTab(&m.activeTab, 3, -1)
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	case "]", "l":
		CycleTab(&m.activeTab, 3, 1)
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	}

	// Activate prefix mode
	switch key {
	case ",", "d", "g", "s":
		if m.getTaskAtCursor() != nil || m.getProjectAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	case "z":
		m.pendingPrefix = "z"
		return m, nil
	case "c", "m":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	}

	// Navigation
	if handleListNavigation(key, &m.cursor, m.totalItems) {
		return m, nil
	}

	switch key {
	case "esc":
		if m.focusedSection != nil {
			m.focusedSection = nil
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseWeeklyReviewMsg{}
		}

	case "q":
		return m, func() tea.Msg {
			return CloseWeeklyReviewMsg{}
		}

	case "enter":
		return m.handleEnter()

	// Section navigation
	case "n":
		m.focusedSection = nil
		m.calculateNavigation()
		return m.goToNextSection()
	case "N":
		m.focusedSection = nil
		m.calculateNavigation()
		return m.goToPrevSection()

	// Toggle active flag (task only)
	case "*":
		return taskOpsToggleActive(m)

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes (task only)
	case "1", "2", "3", "4", "5":
		return taskOpsChangePriority(m, key)
	}

	// Common task actions (tags, snooze, delete, edit, refresh, copy, help)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
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

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *WeeklyReviewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

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
			var targetSection *WeeklyReviewSection
			switch k {
			case "p":
				s := WeeklySectionStaleProjects
				targetSection = &s
			case "i":
				s := WeeklySectionInboxBacklog
				targetSection = &s
			case "b":
				s := WeeklySectionBlockedWork
				targetSection = &s
			case "o":
				s := WeeklySectionOvercommitted
				targetSection = &s
			case "t":
				s := WeeklySectionStaleTasks
				targetSection = &s
			case "m":
				s := WeeklySectionPriorityMismatch
				targetSection = &s
			case "w":
				s := WeeklySectionUpcomingWeek
				targetSection = &s
			}
			if targetSection != nil {
				model, cmd := m.toggleSectionFocus(*targetSection)
				return model, cmd, true
			}
			return m, nil, true

		case "g":
			// WeeklyReview: gp also supports project navigation
			if k == "p" {
				if wp := m.getProjectAtCursor(); wp != nil {
					return m, func() tea.Msg {
						return WeeklyReviewSelectProjectMsg{Project: wp.Project}
					}, true
				}
			}
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	return m, nil
}

// HasFocusedSection returns true if viewing a single focused section.
func (m *WeeklyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// ShouldCaptureEsc returns true when prefix mode, section focus, or confirm dialog is active.
func (m *WeeklyReviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasFocusedSection() || m.confirm.IsActive()
}

func (m *WeeklyReviewModel) viewID() ViewType { return ViewWeeklyReview }
func (m *WeeklyReviewModel) viewName() string { return "Weekly Review" }

// GetHelpItems returns help items for this view.
func (m *WeeklyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewWeeklyReview)
}
