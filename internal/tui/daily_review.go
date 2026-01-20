package tui

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// DailyReviewTask wraps a task with its parent project for cross-project display.
type DailyReviewTask struct {
	Task    *models.Task
	Project *models.Project
}

// DailyReviewSection represents the different sections in daily review.
type DailyReviewSection int

const (
	SectionCompletedRecently DailyReviewSection = iota
	SectionOverdue
	SectionDueToday
	SectionInProgress
	SectionBlocked
	SectionThisWeek
)

// DailyReviewModel displays aggregated task information for daily planning.
type DailyReviewModel struct {
	config *config.Config

	// Sections
	completedRecently []DailyReviewTask
	overdue           []DailyReviewTask
	dueToday          []DailyReviewTask
	inProgress        []DailyReviewTask
	blocked           []DailyReviewTask
	thisWeek          []DailyReviewTask

	// Navigation
	cursor            int
	totalTasks        int
	sectionStarts     []int                // Starting index of each section
	collapsedSections map[int]bool         // Track collapsed sections
	sectionOrder      []DailyReviewSection // Order of non-empty sections

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

// loadTasks aggregates tasks from all active projects into sections.
func (m *DailyReviewModel) loadTasks() {
	m.completedRecently = nil
	m.overdue = nil
	m.dueToday = nil
	m.inProgress = nil
	m.blocked = nil
	m.thisWeek = nil

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekEnd := today.AddDate(0, 0, 7)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			dt := DailyReviewTask{Task: task, Project: project}

			// Completed in last 24 hours
			if task.Status.IsComplete() && task.CompletedAt != nil {
				if task.CompletedAt.After(yesterday) {
					m.completedRecently = append(m.completedRecently, dt)
				}
				continue
			}

			// Skip other completed tasks
			if task.Status.IsComplete() {
				continue
			}

			// In Progress
			if task.Status == models.StatusInProgress {
				m.inProgress = append(m.inProgress, dt)
			}

			// Blocked (status or incomplete dependencies)
			if task.Status == models.StatusBlocked || project.HasIncompleteDependencies(task) {
				m.blocked = append(m.blocked, dt)
				continue // Don't double-count in other sections
			}

			// Due date categorization
			if task.DueDate != nil {
				dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

				if dueDay.Before(today) {
					m.overdue = append(m.overdue, dt)
				} else if dueDay.Equal(today) {
					m.dueToday = append(m.dueToday, dt)
				} else if dueDay.Before(weekEnd) {
					m.thisWeek = append(m.thisWeek, dt)
				}
			}
		}
	}

	// Sort sections
	m.sortSections()
	m.calculateNavigation()
}

// sortSections sorts each section by appropriate criteria.
func (m *DailyReviewModel) sortSections() {
	// Completed: most recent first
	sort.Slice(m.completedRecently, func(i, j int) bool {
		if m.completedRecently[i].Task.CompletedAt == nil {
			return false
		}
		if m.completedRecently[j].Task.CompletedAt == nil {
			return true
		}
		return m.completedRecently[i].Task.CompletedAt.After(*m.completedRecently[j].Task.CompletedAt)
	})

	// Overdue: oldest first (most urgent)
	sort.Slice(m.overdue, func(i, j int) bool {
		return m.overdue[i].Task.DueDate.Before(*m.overdue[j].Task.DueDate)
	})

	// Due today: by priority
	sort.Slice(m.dueToday, func(i, j int) bool {
		return m.dueToday[i].Task.Priority < m.dueToday[j].Task.Priority
	})

	// In progress: by priority then by updated date
	sort.Slice(m.inProgress, func(i, j int) bool {
		if m.inProgress[i].Task.Priority != m.inProgress[j].Task.Priority {
			return m.inProgress[i].Task.Priority < m.inProgress[j].Task.Priority
		}
		return m.inProgress[i].Task.UpdatedAt.After(m.inProgress[j].Task.UpdatedAt)
	})

	// Blocked: by priority
	sort.Slice(m.blocked, func(i, j int) bool {
		return m.blocked[i].Task.Priority < m.blocked[j].Task.Priority
	})

	// This week: by due date then priority
	sort.Slice(m.thisWeek, func(i, j int) bool {
		if !m.thisWeek[i].Task.DueDate.Equal(*m.thisWeek[j].Task.DueDate) {
			return m.thisWeek[i].Task.DueDate.Before(*m.thisWeek[j].Task.DueDate)
		}
		return m.thisWeek[i].Task.Priority < m.thisWeek[j].Task.Priority
	})
}

// calculateNavigation builds navigation indices including section headers.
// Each section header is a navigable item, followed by its tasks (if expanded).
func (m *DailyReviewModel) calculateNavigation() {
	m.sectionOrder = nil
	m.sectionStarts = nil
	m.totalTasks = 0

	sections := []struct {
		section DailyReviewSection
		tasks   []DailyReviewTask
	}{
		{SectionOverdue, m.overdue},
		{SectionDueToday, m.dueToday},
		{SectionInProgress, m.inProgress},
		{SectionBlocked, m.blocked},
		{SectionThisWeek, m.thisWeek},
		{SectionCompletedRecently, m.completedRecently},
	}

	for _, s := range sections {
		if len(s.tasks) > 0 {
			m.sectionOrder = append(m.sectionOrder, s.section)
			m.sectionStarts = append(m.sectionStarts, m.totalTasks)
			m.totalTasks++ // Count header as navigable item
			if !m.collapsedSections[int(s.section)] {
				m.totalTasks += len(s.tasks)
			}
		}
	}
}

// getSectionTasks returns the tasks for a given section.
func (m *DailyReviewModel) getSectionTasks(section DailyReviewSection) []DailyReviewTask {
	switch section {
	case SectionCompletedRecently:
		return m.completedRecently
	case SectionOverdue:
		return m.overdue
	case SectionDueToday:
		return m.dueToday
	case SectionInProgress:
		return m.inProgress
	case SectionBlocked:
		return m.blocked
	case SectionThisWeek:
		return m.thisWeek
	}
	return nil
}

// getTaskAtCursor returns the task at the current cursor position.
// Returns nil if cursor is on a section header.
func (m *DailyReviewModel) getTaskAtCursor() *DailyReviewTask {
	if m.totalTasks == 0 || len(m.sectionOrder) == 0 {
		return nil
	}

	pos := 0
	for _, section := range m.sectionOrder {
		// Check if cursor is on section header
		if m.cursor == pos {
			return nil // On header, not a task
		}
		pos++ // Move past header

		if m.collapsedSections[int(section)] {
			continue // No tasks to check
		}

		tasks := m.getSectionTasks(section)
		sectionEnd := pos + len(tasks)

		if m.cursor >= pos && m.cursor < sectionEnd {
			idx := m.cursor - pos
			if idx >= 0 && idx < len(tasks) {
				return &tasks[idx]
			}
		}

		pos = sectionEnd
	}

	return nil
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
	case ",", "d", "g":
		m.pendingPrefix = key
		return m, nil
	}

	switch key {
	case "esc", "q":
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

	// Section navigation
	case "n":
		return m.goToNextSection()
	case "N":
		return m.goToPrevSection()

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

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewDailyReview,
				ViewName:   "Daily Review",
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
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// toggleCurrentSection toggles collapse state for the section containing the cursor.
func (m *DailyReviewModel) toggleCurrentSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) == 0 {
		return m, nil
	}

	// Find which section contains the cursor (header or task within it)
	pos := 0
	for _, section := range m.sectionOrder {
		headerPos := pos
		pos++ // Move past header

		tasks := m.getSectionTasks(section)
		var sectionEnd int
		if m.collapsedSections[int(section)] {
			sectionEnd = pos // Collapsed: only header
		} else {
			sectionEnd = pos + len(tasks)
		}

		if m.cursor >= headerPos && m.cursor < sectionEnd {
			m.collapsedSections[int(section)] = !m.collapsedSections[int(section)]
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}

		pos = sectionEnd
	}

	return m, nil
}

// goToNextSection moves cursor to next section header (with wrap).
func (m *DailyReviewModel) goToNextSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) <= 1 {
		return m, nil
	}

	// Find current section index
	currentSectionIdx := m.getCurrentSectionIndex()

	// Move to next section (with wrap)
	nextSectionIdx := (currentSectionIdx + 1) % len(m.sectionOrder)
	m.cursor = m.sectionStarts[nextSectionIdx]

	return m, nil
}

// goToPrevSection moves cursor to previous section header (with wrap).
func (m *DailyReviewModel) goToPrevSection() (tea.Model, tea.Cmd) {
	if len(m.sectionOrder) <= 1 {
		return m, nil
	}

	// Find current section index
	currentSectionIdx := m.getCurrentSectionIndex()

	// Move to previous section (with wrap)
	prevSectionIdx := currentSectionIdx - 1
	if prevSectionIdx < 0 {
		prevSectionIdx = len(m.sectionOrder) - 1
	}
	m.cursor = m.sectionStarts[prevSectionIdx]

	return m, nil
}

// getCurrentSectionIndex returns the index of the section containing the cursor.
func (m *DailyReviewModel) getCurrentSectionIndex() int {
	for i := len(m.sectionStarts) - 1; i >= 0; i-- {
		if m.cursor >= m.sectionStarts[i] {
			return i
		}
	}
	return 0
}

// changeTaskStatus changes the status of the task at cursor.
func (m *DailyReviewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	// Skip completed tasks for most status changes
	if task.Status.IsComplete() && newStatus != models.StatusTodo {
		m.statusMessage = "Cannot change status of completed task"
		return m, nil
	}

	// Validate status transition
	switch newStatus {
	case models.StatusInProgress:
		if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
			m.statusMessage = "Task must be todo or blocked to start"
			return m, nil
		}
	case models.StatusDone:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
		// Check for incomplete dependencies
		if project.HasIncompleteDependencies(task) {
			m.err = fmt.Errorf("cannot complete: has incomplete dependencies")
			return m, nil
		}
	case models.StatusBlocked:
		if !task.Status.IsActive() || task.Status == models.StatusBlocked {
			m.statusMessage = "Task must be active and not already blocked"
			return m, nil
		}
	case models.StatusCancelled:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			m.statusMessage = "Task is already todo"
			return m, nil
		}
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task status
	task.Status = newStatus
	task.UpdatedAt = time.Now()
	if newStatus.IsComplete() {
		now := time.Now()
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus)

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *DailyReviewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	task := dt.Task
	project := dt.Project

	if task.Priority == priority {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task priority
	task.Priority = priority
	task.UpdatedAt = time.Now()

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m, nil
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *DailyReviewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Set due date
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	task.DueDate = &newDate
	task.UpdatedAt = time.Now()

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	label := "today"
	if daysFromNow == 1 {
		label = "tomorrow"
	} else if daysFromNow == 7 {
		label = "next week"
	} else if daysFromNow == 30 {
		label = "next month"
	}
	m.statusMessage = fmt.Sprintf("Due date set to %s", label)

	return m, nil
}

// clearDueDate clears the due date from the task at cursor.
func (m *DailyReviewModel) clearDueDate() (tea.Model, tea.Cmd) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return m, nil
	}

	task := dt.Task
	project := dt.Project

	if task.DueDate == nil {
		m.statusMessage = "Task has no due date"
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Clear due date
	task.DueDate = nil
	task.UpdatedAt = time.Now()

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *DailyReviewModel) clampCursor() {
	if m.cursor >= m.totalTasks {
		m.cursor = m.totalTasks - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *DailyReviewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}

// getTotalTaskCount returns count across all sections (ignoring collapse state).
func (m *DailyReviewModel) getTotalTaskCount() int {
	return len(m.completedRecently) + len(m.overdue) + len(m.dueToday) +
		len(m.inProgress) + len(m.blocked) + len(m.thisWeek)
}

// getSectionName returns a human-readable name for a section.
func getSectionName(section DailyReviewSection) string {
	switch section {
	case SectionCompletedRecently:
		return "Completed Recently"
	case SectionOverdue:
		return "Overdue"
	case SectionDueToday:
		return "Due Today"
	case SectionInProgress:
		return "In Progress"
	case SectionBlocked:
		return "Blocked"
	case SectionThisWeek:
		return "This Week"
	}
	return ""
}
