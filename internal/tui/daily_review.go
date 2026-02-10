package tui

import (
	"fmt"

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

	// Filter
	filter CommonFilterState

	// Navigation
	cursor            int
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections
	focusedSection    *DailyReviewSection  // nil = all shown, otherwise show only this section
	activeTab         int                  // 0=Due, 1=Active, 2=Stale, 3=Done

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string

	// UI feedback
	statusMessage string
	err           error
	confirm       ConfirmDialog

	// Dimensions
	width  int
	height int
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

	case TagSelectorDoneMsg:
		if !msg.Canceled {
			m.loadTasks()
			m.clampCursor()
			m.statusMessage = "Tags updated"
		}
		return m, nil

	case SnoozeDateChangedMsg:
		m.loadTasks()
		m.clampCursor()
		m.statusMessage = "Snooze date updated"
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
				return m.deleteTask()
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
		if m.activeTab > 0 {
			m.activeTab--
		} else {
			m.activeTab = 3
		}
		m.focusedSection = nil
		m.calculateNavigation()
		m.clampCursor()
		return m, nil
	case "]", "l":
		if m.activeTab < 3 {
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
	case ",", "d", "g", "s", "m":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	case "c":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "c"
		}
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

	// Tag selector
	case "t":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: dt.Task, Project: dt.Project, CurrentTags: append([]string{}, dt.Task.Tags...)}
			}
		}
		return m, nil

	// Snooze picker
	case "Z":
		if dt := m.getTaskAtCursor(); dt != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: dt.Task, Project: dt.Project}
			}
		}
		return m, nil

	// Delete task (with confirmation)
	case "D":
		if dt := m.getTaskAtCursor(); dt != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", dt.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
			return m, nil
		}
		return m, nil

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
	case "F":
		return m.handleDailyReviewFilterPrefix(key)
	case "Ft":
		return m.handleDailyReviewFilterTypePrefix(key)
	case "Fa":
		return m.handleDailyReviewFilterAttrPrefix(key)
	case "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap":
		return m.handleDailyReviewFilterAttrSubPrefix(prefix, key)
	case "FS":
		return m.handleDailyReviewFilterScorePrefix(key)

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
			return m.changeTaskStatus(models.StatusCanceled)
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

	case "m":
		// Move to section
		section, ok := sectionForMoveKey(key)
		if !ok {
			return m, nil
		}
		return m.sectionMove(section)

	case "c":
		// Create linked task or navigate
		dt := m.getTaskAtCursor()
		if dt == nil {
			return m, nil
		}
		task := dt.Task
		project := dt.Project

		// Navigate to linked tasks
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

// ShouldCaptureEsc returns true when prefix mode, section focus, filters, or confirm dialog are active.
func (m *DailyReviewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasFocusedSection() || m.HasActiveFilters() || m.confirm.IsActive()
}

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}
