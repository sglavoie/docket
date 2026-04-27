package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

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
		tabs:              TabState{Names: weeklyReviewTabNames},
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
	key := msg.String()

	// Handle completion summary overlay
	if m.showingSummary {
		if key == "esc" {
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
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
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
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmct.", m.getTaskAtCursor() != nil); ok {
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

	// Toggle review completion / mark section reviewed
	case "R":
		if m.reviewCompleted {
			m.reviewCompleted = false
			m.showingSummary = false
			for k := range m.reviewedSections {
				delete(m.reviewedSections, k)
			}
			// Re-auto-review empty sections
			m.loadData()
			return m, func() tea.Msg { return ResetWeeklyReviewMsg{} }
		}
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
			// Let gg (jump-to-top) fall through to common dispatch
			if k == "g" {
				return nil, nil, false
			}
			// gp: go-to-project (common dispatch handles task gp; override for project rows)
			if k == "p" {
				if wp := m.getProjectAtCursor(); wp != nil {
					return m, func() tea.Msg {
						return WeeklyReviewSelectProjectMsg{Project: wp.Project}
					}, true
				}
				// Fall through to common gp (task go-to-project)
				return nil, nil, false
			}
			// Section-focus sub-keys (moved from s prefix)
			var targetSection *WeeklyReviewSection
			switch k {
			case "P":
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
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		if m.err == nil {
			switch prefix {
			case ",":
				m.trackMutation("status")
			case "!":
				m.trackMutation("priority")
			case "d":
				m.trackMutation("due_date")
			case "s":
				m.trackMutation("snooze")
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

func (m *WeeklyReviewModel) HandledKeys() []string {
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
			"g",
			"zR", "zM", "zp",
			"tab", "n", "N",
			"gP", "gi", "gb", "go", "gt", "gm", "gw", "gp",
			"enter", "m", "t",
			"R", "ctrl+n",
			"1", "2", "3", "4", "5",
			"esc",
		},
	)
}

func (m *WeeklyReviewModel) StatusBarLabel() string { return "Weekly Review" }

// IsFullHeight returns true because the weekly review renders its own footer.
func (m *WeeklyReviewModel) IsFullHeight() bool { return true }
