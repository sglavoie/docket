package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// FocusTask wraps a task with its parent project for cross-project display.
type FocusTask struct {
	Task    *models.Task
	Project *models.Project
}

// FocusViewModel displays all active-flagged tasks across projects.
type FocusViewModel struct {
	config *config.Config

	// Tasks
	tasks []FocusTask

	// Navigation
	cursor int

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string

	// UI feedback
	statusMessage string
	err           error

	// Dimensions
	width  int
	height int
}

// NewFocusViewModel creates a new focus view model.
func NewFocusViewModel(cfg *config.Config) *FocusViewModel {
	m := &FocusViewModel{
		config: cfg,
	}
	m.loadTasks()
	return m
}

// loadTasks aggregates all active-flagged tasks from all active projects.
func (m *FocusViewModel) loadTasks() {
	m.tasks = nil

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if !task.Active {
				continue
			}
			// Skip completed tasks
			if task.Status.IsComplete() {
				continue
			}
			m.tasks = append(m.tasks, FocusTask{Task: task, Project: project})
		}
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Task.Priority != m.tasks[j].Task.Priority {
			return m.tasks[i].Task.Priority < m.tasks[j].Task.Priority
		}
		return m.tasks[i].Task.UpdatedAt.After(m.tasks[j].Task.UpdatedAt)
	})
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *FocusViewModel) getTaskAtCursor() *FocusTask {
	if m.cursor < 0 || m.cursor >= len(m.tasks) {
		return nil
	}
	return &m.tasks[m.cursor]
}

// HasActiveSearch returns false (no search in this view).
func (m *FocusViewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *FocusViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *FocusViewModel) Init() tea.Cmd {
	return nil
}

func (m *FocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case FocusViewRefreshMsg:
		m.loadTasks()
		m.clampCursor()
		return m, nil

	case TaskUpdatedMsg:
		// Auto-refresh when a task is updated (e.g., after editing)
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *FocusViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.statusMessage = ""

	// Activate prefix mode
	switch key {
	case ",", "d", "g":
		m.pendingPrefix = key
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return CloseFocusViewMsg{}
		}

	case "enter":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project}
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
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if len(m.tasks) > 0 {
			m.cursor = len(m.tasks) - 1
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
		if m.cursor >= len(m.tasks) {
			m.cursor = len(m.tasks) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	// Toggle active flag (remove from focus)
	case "*":
		return m.toggleActive()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)

	// Edit task
	case "e":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: ft.Task}
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
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: ft.Task}
			}
		}
		return m, nil

	case "Y":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: ft.Task}
			}
		}
		return m, nil

	case "ctrl+y":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: ft.Task}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewFocusView,
				ViewName:   "Focus View",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
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
			if ft := m.getTaskAtCursor(); ft != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: ft.Task}
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
			if ft := m.getTaskAtCursor(); ft != nil {
				return m, func() tea.Msg {
					return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project}
				}
			}
			return m, nil
		}
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// toggleActive removes the task from focus by clearing its Active flag.
func (m *FocusViewModel) toggleActive() (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	task := ft.Task
	project := ft.Project

	// Backup before change
	if err := storage.BackupProject(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Toggle active flag off
	task.Active = false
	task.UpdatedAt = time.Now()

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Removed #%d from focus", task.ShortID)

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// changeTaskStatus changes the status of the task at cursor.
func (m *FocusViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	task := ft.Task
	project := ft.Project

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
		// Clear active flag when completing
		task.Active = false
	} else {
		task.CompletedAt = nil
	}

	// Save project
	if err := storage.SaveProject(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus)

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *FocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	task := ft.Task
	project := ft.Project

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
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *FocusViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	task := ft.Task
	project := ft.Project

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
	_ = storage.LoadProject(project)
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

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// clearDueDate clears the due date from the task at cursor.
func (m *FocusViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	task := ft.Task
	project := ft.Project

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
	_ = storage.LoadProject(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// clampCursor ensures cursor is within valid range.
func (m *FocusViewModel) clampCursor() {
	if m.cursor >= len(m.tasks) {
		m.cursor = len(m.tasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *FocusViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// GetHelpItems returns help items for this view.
func (m *FocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusView)
}

func (m *FocusViewModel) View() string {
	if len(m.tasks) == 0 {
		var b strings.Builder
		b.WriteString(MutedStyle.Render("No tasks in focus."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Open a project and mark tasks as active with *."))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press q to return to dashboard."))

		return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Focus View",
			Subtitle: "no active tasks",
			Centered: true,
		})
	}

	var b strings.Builder

	// Calculate available height for task list
	contentHeight := m.height - 12

	// Render task lines
	var lines []string
	maxTitleLen := m.width - 45

	for i, ft := range m.tasks {
		isSelected := i == m.cursor
		line := m.renderTaskLine(ft, isSelected, maxTitleLen)
		lines = append(lines, line)
	}

	// Apply scrolling if needed
	scrollOffset := 0
	if m.cursor >= contentHeight {
		scrollOffset = m.cursor - contentHeight + 1
	}

	visibleLines := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visibleLines = lines[scrollOffset:]
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	b.WriteString(strings.Join(visibleLines, "\n"))
	b.WriteString("\n\n")

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
		b.WriteString("\n")
	}

	// Status message
	if m.statusMessage != "" {
		b.WriteString(SuccessStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	// Error message
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}

	// Footer
	footer := HelpStyle.Render("j/k nav | * unfocus | ,s/d/b/x status | 1-5 priority | dt/dm due | Enter go to | ? help")
	b.WriteString(footer)

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Focus View",
		Subtitle: fmt.Sprintf("%d active tasks", len(m.tasks)),
		Centered: true,
	})
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *FocusViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": FocusViewNavigationPrefixHints,
	}, m.width)
}

// FocusViewNavigationPrefixHints contains "g" prefix hints for focus view.
var FocusViewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first"},
	{Key: "p", Description: "go to project"},
}

// renderTaskLine renders a single task line.
func (m *FocusViewModel) renderTaskLine(ft FocusTask, isSelected bool, maxTitleLen int) string {
	task := ft.Task

	// Cursor
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Project name
	projectName := util.Truncate(ft.Project.Name, 12)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	// Status icon
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	// Type icon
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Priority badge
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Tags (show first 2)
	tagsStr := ""
	if len(task.Tags) > 0 {
		tags := task.Tags
		if len(tags) > 2 {
			tags = tags[:2]
		}
		tagsStr = MutedStyle.Render(" #" + strings.Join(tags, " #"))
	}

	// Due date
	dueStr := ""
	if task.DueDate != nil {
		dueStr = m.formatRelativeDue(task.DueDate)
	}

	// Build line
	line := fmt.Sprintf("%s%s %s %s %s %s%s", cursor, projectStr, statusIcon, typeIcon, priorityBadge, title, tagsStr)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderPriorityBadge renders a priority indicator.
func (m *FocusViewModel) renderPriorityBadge(priority int) string {
	badge := fmt.Sprintf("!%d", priority)
	switch priority {
	case 1:
		return PriorityHighStyle.Render(badge)
	case 2:
		return PriorityMediumStyle.Render(badge)
	default:
		return PriorityLowStyle.Render(badge)
	}
}

// formatRelativeDue formats the due date for display.
func (m *FocusViewModel) formatRelativeDue(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return OverdueStyle.Render(fmt.Sprintf("%d days ago", -daysUntil))
	case daysUntil == -1:
		return OverdueStyle.Render("yesterday")
	case daysUntil == 0:
		return DueTodayStyle.Render("today")
	case daysUntil == 1:
		return DueDateStyle.Render("tomorrow")
	case daysUntil < 7:
		return DueDateStyle.Render(dueDate.Format("Mon"))
	default:
		if dueDate.Year() != now.Year() {
			return DueDateStyle.Render(dueDate.Format("Jan 2 '06"))
		}
		return DueDateStyle.Render(dueDate.Format("Jan 2"))
	}
}
