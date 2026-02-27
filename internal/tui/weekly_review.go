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
	WeeklySectionRecurringOverview
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
	ViewBundle

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

	// Navigation
	totalItems        int
	sectionStarts     []int
	collapsedSections map[int]bool
	sectionOrder      []WeeklyReviewSection
	focusedSection    *WeeklyReviewSection
	tabs              TabState // 0=Projects, 1=Tasks, 2=Planning
	scrollOffset      int

	// Guided review flow
	reviewedSections map[WeeklyReviewSection]bool
	mutationCounts   map[string]int
	showingSummary   bool
}

// NewWeeklyReviewModel creates a new weekly review model.
func NewWeeklyReviewModel(cfg *config.Config, mgr *config.Manager) *WeeklyReviewModel {
	m := &WeeklyReviewModel{
		ViewBundle:        ViewBundle{config: cfg, configManager: mgr},
		collapsedSections: make(map[int]bool),
		reviewedSections:  make(map[WeeklyReviewSection]bool),
		mutationCounts:    make(map[string]int),
		tabs:              TabState{Names: weeklyReviewTabNames},
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
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
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

	// Handle completion summary overlay
	if m.showingSummary {
		if key == "q" || key == "esc" {
			return m, func() tea.Msg {
				return CloseWeeklyReviewMsg{}
			}
		}
		m.showingSummary = false
		return m, nil
	}

	// Handle confirmation dialog
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			model, cmd := taskOpsDeleteTask(m)
			if m.err == nil {
				m.trackMutation("delete")
			}
			return model, cmd
		}
		return m, nil
	}

	// Handle pending prefix keys
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
	hasTask := m.getTaskAtCursor() != nil
	hasItem := hasTask || m.getProjectAtCursor() != nil
	if prefix, ok := activateSecondaryPrefix(key, "z", ",dgs", hasItem); ok {
		m.pendingPrefix = prefix
		return m, nil
	}
	if prefix, ok := activateSecondaryPrefix(key, "", "!cm", hasTask); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, m.totalItems, m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
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
	case "ctrl+n":
		return m.goToNextUnreviewed()

	// Mark section reviewed
	case "R":
		return m.markCurrentSectionReviewed()

	// Toggle active flag (task only)
	case "*":
		model, cmd := taskOpsToggleActive(m)
		if m.err == nil {
			m.trackMutation("active")
		}
		return model, cmd

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes (task only)
	case "1", "2", "3", "4", "5":
		model, cmd := taskOpsChangePriority(m, key)
		if m.err == nil {
			m.trackMutation("priority")
		}
		return model, cmd
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
			case "r":
				s := WeeklySectionRecurringOverview
				targetSection = &s
			}
			if targetSection != nil {
				model, cmd := m.toggleSectionFocus(*targetSection)
				return model, cmd, true
			}
			return m, nil, true

		case "!":
			if _, ok := PriorityForPrefixKey(k); ok {
				model, cmd := taskOpsChangePriority(m, k)
				if m.err == nil {
					m.trackMutation("priority")
				}
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
		if m.err == nil {
			switch prefix {
			case ",":
				m.trackMutation("status")
			case "d":
				m.trackMutation("due_date")
			case "m":
				m.trackMutation("move")
			}
		}
		return model, cmd
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
	return GetKeybindingRegistry().GetForView(ViewWeeklyReview)
}
