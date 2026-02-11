package tui

import (
	"fmt"

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

	// UI feedback
	err error

	// Dimensions
	width  int
	height int
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

// HasActiveSearch returns false (no search in this view).
func (m *WeeklyReviewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *WeeklyReviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *WeeklyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *WeeklyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case WeeklyReviewRefreshMsg:
		m.loadData()
		m.clampCursor()
		return m, nil

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.loadData()
			m.clampCursor()
			m.statusMessage = "Tags updated"
		}
		return m, nil

	case SnoozeDateChangedMsg:
		m.loadData()
		m.clampCursor()
		m.statusMessage = "Snooze date updated"
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
				return m.deleteTask()
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
		if m.activeTab > 0 {
			m.activeTab--
		} else {
			m.activeTab = 2
		}
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	case "]", "l":
		if m.activeTab < 2 {
			m.activeTab++
		} else {
			m.activeTab = 0
		}
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

	// Navigation
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < m.totalItems-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if m.totalItems > 0 {
			m.cursor = m.totalItems - 1
		}
		return m, nil

	case "ctrl+u":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	case "ctrl+d":
		m.cursor += 10
		if m.cursor >= m.totalItems {
			m.cursor = m.totalItems - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

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
		return m.toggleActive()

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes (task only)
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)

	// Edit task
	case "e":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: dt.Task}
			}
		}
		return m, nil

	// Refresh
	case "r":
		m.loadData()
		m.clampCursor()
		m.statusMessage = "Refreshed"
		return m, nil

	// Copy
	case "y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: dt.Task}
			}
		}
		return m, nil

	case "Y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: dt.Task}
			}
		}
		return m, nil

	case "ctrl+y":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: dt.Task}
			}
		}
		return m, nil

	// Tag selector
	case "t":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: dt.Task, Project: dt.Project, CurrentTags: append([]string{}, dt.Task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: dt.Task, Project: dt.Project}
			}
		}
		return m, nil

	// Delete task
	case "D":
		if dt := m.getTaskAtCursor(); dt != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", dt.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewWeeklyReview,
				ViewName:   "Weekly Review",
				Items:      m.GetHelpItems(),
			}
		}
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

	switch prefix {
	case "z":
		switch key {
		case "R":
			// Expand all sections
			m.collapsedSections = make(map[int]bool)
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		case "M":
			// Collapse all visible sections
			for _, section := range m.sectionOrder {
				m.collapsedSections[int(section)] = true
			}
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}
		return m, nil

	case ",":
		if status, ok := StatusForPrefixKey(key); ok {
			return m.changeTaskStatus(status)
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if isClear {
				return m.clearDueDate()
			}
			return m.setQuickDueDate(days)
		}
		if key == "d" {
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: dt.Task}
				}
			}
		}

	case "g":
		switch key {
		case "g":
			m.cursor = 0
			return m, nil
		case "p":
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return WeeklyReviewSelectTaskMsg{Task: dt.Task, Project: dt.Project}
				}
			}
			if wp := m.getProjectAtCursor(); wp != nil {
				return m, func() tea.Msg {
					return WeeklyReviewSelectProjectMsg{Project: wp.Project}
				}
			}
			return m, nil
		}

	case "s":
		var targetSection *WeeklyReviewSection
		switch key {
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
			return m.toggleSectionFocus(*targetSection)
		}

	case "c":
		dt := m.getTaskAtCursor()
		if dt == nil {
			return m, nil
		}
		task := dt.Task
		project := dt.Project

		if key == "n" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
			}
		}

		var linkType LinkType
		switch key {
		case "b":
			linkType = LinkTypeBlocker
		case "d":
			linkType = LinkTypeDependent
		case "s":
			linkType = LinkTypeSubtask
		case "f":
			linkType = LinkTypeFollowUp
		default:
			return m, nil
		}
		return m, func() tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: linkType}
		}

	case "m":
		section, ok := sectionForMoveKey(key)
		if !ok {
			return m, nil
		}
		return m.sectionMove(section)
	}

	return m, nil
}

// HasFocusedSection returns true if viewing a single focused section.
func (m *WeeklyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// HasTextInput returns false for the weekly review view.
func (m *WeeklyReviewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode, section focus, or confirm dialog is active.
func (m *WeeklyReviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasFocusedSection() || m.confirm.IsActive()
}

// GetHelpItems returns help items for this view.
func (m *WeeklyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewWeeklyReview)
}
