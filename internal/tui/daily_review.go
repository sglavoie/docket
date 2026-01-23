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

// loadTasks aggregates tasks from all active projects into sections.
func (m *DailyReviewModel) loadTasks() {
	m.overdue = nil
	m.dueToday = nil
	m.dueSoon = nil
	m.dueLater = nil
	m.future = nil
	m.inProgress = nil
	m.blocked = nil
	m.old = nil
	m.completed = nil

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get configurable thresholds
	dueSoonDays := m.config.GetReviewDueSoonDays()
	futureDays := m.config.GetReviewFutureDays()
	oldDays := m.config.GetReviewOldDays()
	completedHours := m.config.GetReviewCompletedHours()

	// Calculate threshold dates
	// Due Soon: tomorrow through dueSoonDays (e.g., days 1-7)
	dueSoonEnd := today.AddDate(0, 0, dueSoonDays+1)
	// Future: tasks due in >= futureDays (e.g., 300+ days)
	futureStart := today.AddDate(0, 0, futureDays)
	// Due Later: between Due Soon and Future (e.g., days 8-299)
	oldThreshold := now.AddDate(0, 0, -oldDays)
	completedThreshold := now.Add(-time.Duration(completedHours) * time.Hour)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			dt := DailyReviewTask{Task: task, Project: project}

			// 1. Completed: CompletedAt within completedHours
			if task.Status.IsComplete() && task.CompletedAt != nil {
				if completedHours > 0 && task.CompletedAt.After(completedThreshold) {
					m.completed = append(m.completed, dt)
				}
				continue
			}

			// 2. Skip other completed tasks
			if task.Status.IsComplete() {
				continue
			}

			// 3. Overdue: DueDate < today
			if task.DueDate != nil {
				dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

				if dueDay.Before(today) {
					m.overdue = append(m.overdue, dt)
					continue
				}

				// 4. Today: DueDate == today
				if dueDay.Equal(today) {
					m.dueToday = append(m.dueToday, dt)
					continue
				}

				// 5. Due Soon: DueDate between tomorrow and dueSoonDays (e.g., 1-7 days)
				if dueSoonDays > 0 && dueDay.Before(dueSoonEnd) {
					m.dueSoon = append(m.dueSoon, dt)
					continue
				}

				// 6. Future: DueDate >= futureDays (e.g., 300+ days)
				if futureDays > 0 && !dueDay.Before(futureStart) {
					m.future = append(m.future, dt)
					continue
				}

				// 7. Due Later: DueDate between dueSoonDays+1 and futureDays-1 (e.g., 8-299 days)
				// This catches everything between Due Soon and Future
				m.dueLater = append(m.dueLater, dt)
				continue
			}

			// 8. Blocked: status == blocked or has incomplete deps
			if task.Status == models.StatusBlocked || project.HasIncompleteDependencies(task) {
				m.blocked = append(m.blocked, dt)
				continue
			}

			// 9. In Progress: status == in_progress
			if task.Status == models.StatusInProgress {
				m.inProgress = append(m.inProgress, dt)
				continue
			}

			// 10. Old: UpdatedAt older than oldDays
			if oldDays > 0 && task.UpdatedAt.Before(oldThreshold) {
				m.old = append(m.old, dt)
			}
		}
	}

	// Sort sections
	m.sortSections()
	m.calculateNavigation()
}

// sortSections sorts each section by appropriate criteria.
func (m *DailyReviewModel) sortSections() {
	// Overdue: oldest first (most urgent)
	sort.Slice(m.overdue, func(i, j int) bool {
		return m.overdue[i].Task.DueDate.Before(*m.overdue[j].Task.DueDate)
	})

	// Due today: by priority
	sort.Slice(m.dueToday, func(i, j int) bool {
		return m.dueToday[i].Task.Priority < m.dueToday[j].Task.Priority
	})

	// Due soon: by due date then priority
	sort.Slice(m.dueSoon, func(i, j int) bool {
		if !m.dueSoon[i].Task.DueDate.Equal(*m.dueSoon[j].Task.DueDate) {
			return m.dueSoon[i].Task.DueDate.Before(*m.dueSoon[j].Task.DueDate)
		}
		return m.dueSoon[i].Task.Priority < m.dueSoon[j].Task.Priority
	})

	// Due later: by due date then priority
	sort.Slice(m.dueLater, func(i, j int) bool {
		if !m.dueLater[i].Task.DueDate.Equal(*m.dueLater[j].Task.DueDate) {
			return m.dueLater[i].Task.DueDate.Before(*m.dueLater[j].Task.DueDate)
		}
		return m.dueLater[i].Task.Priority < m.dueLater[j].Task.Priority
	})

	// Future: by due date then priority
	sort.Slice(m.future, func(i, j int) bool {
		if !m.future[i].Task.DueDate.Equal(*m.future[j].Task.DueDate) {
			return m.future[i].Task.DueDate.Before(*m.future[j].Task.DueDate)
		}
		return m.future[i].Task.Priority < m.future[j].Task.Priority
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

	// Old: by UpdatedAt (oldest first)
	sort.Slice(m.old, func(i, j int) bool {
		return m.old[i].Task.UpdatedAt.Before(m.old[j].Task.UpdatedAt)
	})

	// Completed: most recent first
	sort.Slice(m.completed, func(i, j int) bool {
		if m.completed[i].Task.CompletedAt == nil {
			return false
		}
		if m.completed[j].Task.CompletedAt == nil {
			return true
		}
		return m.completed[i].Task.CompletedAt.After(*m.completed[j].Task.CompletedAt)
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
		{SectionDueSoon, m.dueSoon},
		{SectionDueLater, m.dueLater},
		{SectionFuture, m.future},
		{SectionInProgress, m.inProgress},
		{SectionBlocked, m.blocked},
		{SectionOld, m.old},
		{SectionCompleted, m.completed},
	}

	for _, s := range sections {
		if len(s.tasks) > 0 {
			// If focused on a section, only show that section
			if m.focusedSection != nil && *m.focusedSection != s.section {
				continue
			}
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
	case SectionOverdue:
		return m.overdue
	case SectionDueToday:
		return m.dueToday
	case SectionDueSoon:
		return m.dueSoon
	case SectionDueLater:
		return m.dueLater
	case SectionFuture:
		return m.future
	case SectionInProgress:
		return m.inProgress
	case SectionBlocked:
		return m.blocked
	case SectionOld:
		return m.old
	case SectionCompleted:
		return m.completed
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

// toggleActive toggles the Active flag on the task at cursor.
func (m *DailyReviewModel) toggleActive() (tea.Model, tea.Cmd) {
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

	// Toggle active flag
	task.Active = !task.Active
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

	if task.Active {
		m.statusMessage = fmt.Sprintf("Task #%d marked active", task.ShortID)
	} else {
		m.statusMessage = fmt.Sprintf("Task #%d unmarked active", task.ShortID)
	}

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// toggleSectionFocus toggles focus on a specific section.
// If already focused on that section, shows all sections.
// Otherwise, focuses only on that section.
func (m *DailyReviewModel) toggleSectionFocus(section DailyReviewSection) (tea.Model, tea.Cmd) {
	if m.focusedSection != nil && *m.focusedSection == section {
		// Already focused on this section, show all
		m.focusedSection = nil
	} else {
		// Focus on this section
		m.focusedSection = &section
	}
	m.calculateNavigation()
	m.clampCursor()
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

// HasFocusedSection returns true if viewing a single focused section.
func (m *DailyReviewModel) HasFocusedSection() bool {
	return m.focusedSection != nil
}

// GetHelpItems returns help items for this view.
func (m *DailyReviewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewDailyReview)
}

// getTotalTaskCount returns count across all sections (ignoring collapse state).
func (m *DailyReviewModel) getTotalTaskCount() int {
	return len(m.overdue) + len(m.dueToday) + len(m.dueSoon) + len(m.dueLater) +
		len(m.future) + len(m.inProgress) + len(m.blocked) + len(m.old) + len(m.completed)
}

// getSectionName returns a human-readable name for a section.
func getSectionName(section DailyReviewSection) string {
	switch section {
	case SectionOverdue:
		return "Overdue"
	case SectionDueToday:
		return "Today"
	case SectionDueSoon:
		return "Due Soon"
	case SectionDueLater:
		return "Due Later"
	case SectionFuture:
		return "Future"
	case SectionInProgress:
		return "In Progress"
	case SectionBlocked:
		return "Blocked"
	case SectionOld:
		return "Old"
	case SectionCompleted:
		return "Completed"
	}
	return ""
}
