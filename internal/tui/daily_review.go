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
	SectionOld
	SectionCompleted
)

// DailyReviewModel displays aggregated task information for review.
type DailyReviewModel struct {
	config *config.Config

	// Sections
	overdue    []DailyReviewTask
	dueToday   []DailyReviewTask
	dueSoon    []DailyReviewTask
	dueLater   []DailyReviewTask
	future     []DailyReviewTask
	inProgress []DailyReviewTask
	blocked    []DailyReviewTask
	old        []DailyReviewTask
	completed  []DailyReviewTask

	// Navigation
	cursor            int
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections
	focusedSection    *DailyReviewSection  // nil = all shown, otherwise show only this section

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string

	// UI feedback
	statusMessage string
	err           error

	// Dimensions
	width  int
	height int
}

// NewDailyReviewModel creates a new daily review model.
func NewDailyReviewModel(cfg *config.Config) *DailyReviewModel {
	m := &DailyReviewModel{
		config:            cfg,
		collapsedSections: make(map[int]bool),
	}
	m.loadTasks()
	return m
}

// HasActiveSearch returns false (no search in this view).
func (m *DailyReviewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *DailyReviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DailyReviewModel) Init() tea.Cmd {
	return nil
}

func (m *DailyReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

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

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil

	// Activate prefix mode
	switch key {
	case ",", "d", "g", "s":
		m.pendingPrefix = key
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
		return m, func() tea.Msg {
			return CloseDailyReviewMsg{}
		}

	case "q":
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

	// Navigation
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < m.totalTasks-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if m.totalTasks > 0 {
			m.cursor = m.totalTasks - 1
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
		if m.cursor >= m.totalTasks {
			m.cursor = m.totalTasks - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
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
		return m.toggleActive()

	// Toggle section collapse
	case "tab":
		return m.toggleCurrentSection()

	// Priority changes
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
		m.loadTasks()
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

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewDailyReview,
				ViewName:   "Review",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *DailyReviewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		// Status operations
		switch key {
		case "s":
			return m.changeTaskStatus(models.StatusInProgress)
		case "d":
			return m.changeTaskStatus(models.StatusDone)
		case "b":
			return m.changeTaskStatus(models.StatusBlocked)
		case "x":
			return m.changeTaskStatus(models.StatusCancelled)
		case "o":
			return m.changeTaskStatus(models.StatusTodo)
		}

	case "d":
		// Due date operations
		switch key {
		case "t":
			return m.setQuickDueDate(0) // Today
		case "m":
			return m.setQuickDueDate(1) // Tomorrow
		case "w":
			return m.setQuickDueDate(7) // +1 week
		case "n":
			return m.setQuickDueDate(30) // +1 month
		case "0":
			return m.clearDueDate()
		case "d":
			// Open date picker
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: dt.Task}
				}
			}
		}

	case "g":
		switch key {
		case "g":
			// gg: Go to first item
			m.cursor = 0
			return m, nil
		case "p":
			// gp: Go to project (navigate to task in its project)
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return DailyReviewSelectMsg{Task: dt.Task, Project: dt.Project}
				}
			}
			return m, nil
		}

	case "s":
		// Section focus toggles
		var targetSection *DailyReviewSection
		switch key {
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
		case "o":
			s := SectionOld
			targetSection = &s
		}
		if targetSection != nil {
			return m.toggleSectionFocus(*targetSection)
		}
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *DailyReviewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasFocusedSection returns true if viewing a single focused section.
func (m *DailyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// HasTextInput returns false for the daily review view.
func (m *DailyReviewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode or section focus is active.
func (m *DailyReviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasFocusedSection()
}

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}
