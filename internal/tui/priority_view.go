package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// PriorityGroup represents a category of tasks by urgency.
type PriorityGroup int

const (
	PriorityGroupOverdue PriorityGroup = iota
	PriorityGroupToday
	PriorityGroupThisWeek
	PriorityGroupLater
	PriorityGroupBacklog
	PriorityGroupWaitingOn
)

// PriorityTask wraps a task with computed urgency metadata.
type PriorityTask struct {
	Task         *models.Task
	UrgencyScore int
	ParentTitle  string // Cached parent context
	RelativeDue  string // Formatted relative due date
}

// PriorityViewModel displays actionable tasks sorted by urgency.
type PriorityViewModel struct {
	project *models.Project

	// Grouped tasks by urgency
	overdue   []*PriorityTask
	today     []*PriorityTask
	thisWeek  []*PriorityTask
	later     []*PriorityTask
	backlog   []*PriorityTask // Tasks without due dates
	waitingOn []*PriorityTask // Blocked tasks

	// Navigation
	cursor      int   // Absolute position across all visible groups
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Waiting On section state
	waitingOnCollapsed bool

	// Dimensions
	width  int
	height int
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project) *PriorityViewModel {
	m := &PriorityViewModel{
		project:            project,
		waitingOnCollapsed: true,
	}
	m.loadActionableTasks()
	return m
}

// loadActionableTasks categorizes and sorts tasks by urgency.
func (m *PriorityViewModel) loadActionableTasks() {
	m.overdue = nil
	m.today = nil
	m.thisWeek = nil
	m.later = nil
	m.backlog = nil
	m.waitingOn = nil

	for _, task := range m.project.Tasks {
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}

		// Check if blocked (status=blocked or has incomplete dependencies)
		isBlocked := task.Status == models.StatusBlocked || m.project.HasIncompleteDependencies(task)

		pt := &PriorityTask{
			Task:         task,
			UrgencyScore: m.computeUrgencyScore(task),
			ParentTitle:  m.getParentTitle(task),
			RelativeDue:  m.formatRelativeDue(task),
		}

		if isBlocked {
			m.waitingOn = append(m.waitingOn, pt)
			continue
		}

		// Categorize by due date
		group := m.categorizeByDueDate(task)
		switch group {
		case PriorityGroupOverdue:
			m.overdue = append(m.overdue, pt)
		case PriorityGroupToday:
			m.today = append(m.today, pt)
		case PriorityGroupThisWeek:
			m.thisWeek = append(m.thisWeek, pt)
		case PriorityGroupLater:
			m.later = append(m.later, pt)
		case PriorityGroupBacklog:
			m.backlog = append(m.backlog, pt)
		}
	}

	// Sort each group by urgency score (higher = more urgent)
	sortByUrgency := func(tasks []*PriorityTask) {
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].UrgencyScore > tasks[j].UrgencyScore
		})
	}
	sortByUrgency(m.overdue)
	sortByUrgency(m.today)
	sortByUrgency(m.thisWeek)
	sortByUrgency(m.later)
	sortByUrgency(m.backlog)
	sortByUrgency(m.waitingOn)

	// Calculate navigation indices (excluding waitingOn which is collapsible)
	m.totalTasks = len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later) + len(m.backlog)
	m.groupStarts = []int{
		0,
		len(m.overdue),
		len(m.overdue) + len(m.today),
		len(m.overdue) + len(m.today) + len(m.thisWeek),
		len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later),
	}
}

// computeUrgencyScore calculates the urgency score for a task.
// Higher scores = more urgent.
func (m *PriorityViewModel) computeUrgencyScore(task *models.Task) int {
	score := 0

	// Priority weight: priority 1 = 5000, priority 5 = 1000
	priorityWeight := (6 - task.Priority) * 1000
	score += priorityWeight

	// Bug type bonus: Bugs at the same priority rank higher
	if task.Type == models.TaskTypeBug {
		score += 100
	}

	// Due date proximity: overdue adds points, future subtracts
	if task.DueDate != nil {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
		daysUntil := int(dueDay.Sub(today).Hours() / 24)
		score += (-daysUntil) * 50
	}

	// In-progress bonus
	if task.Status == models.StatusInProgress {
		score += 500
	}

	// Active flag bonus
	if task.Active {
		score += 200
	}

	return score
}

// categorizeByDueDate determines which urgency group a task belongs to.
func (m *PriorityViewModel) categorizeByDueDate(task *models.Task) PriorityGroup {
	if task.DueDate == nil {
		return PriorityGroupBacklog
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		return PriorityGroupOverdue
	}
	if dueDay.Equal(today) {
		return PriorityGroupToday
	}
	weekEnd := today.AddDate(0, 0, 7)
	if dueDay.Before(weekEnd) {
		return PriorityGroupThisWeek
	}
	return PriorityGroupLater
}

// getParentTitle returns the parent task's title for context.
func (m *PriorityViewModel) getParentTitle(task *models.Task) string {
	if task.ParentID == nil {
		return ""
	}
	if parent := m.project.GetTaskByID(*task.ParentID); parent != nil {
		return parent.Title
	}
	return ""
}

// formatRelativeDue formats the due date for display.
func (m *PriorityViewModel) formatRelativeDue(task *models.Task) string {
	if task.DueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return task.DueDate.Format("Mon")
	default:
		if task.DueDate.Year() != now.Year() {
			return task.DueDate.Format("Jan 2 '06")
		}
		return task.DueDate.Format("Jan 2")
	}
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *PriorityViewModel) getTaskAtCursor() *PriorityTask {
	if m.totalTasks == 0 {
		return nil
	}

	cursor := m.cursor
	if cursor < m.groupStarts[1] {
		if cursor < len(m.overdue) {
			return m.overdue[cursor]
		}
		return nil
	}
	cursor -= m.groupStarts[1]
	if cursor < len(m.today) {
		return m.today[cursor]
	}
	cursor -= len(m.today)
	if cursor < len(m.thisWeek) {
		return m.thisWeek[cursor]
	}
	cursor -= len(m.thisWeek)
	if cursor < len(m.later) {
		return m.later[cursor]
	}
	cursor -= len(m.later)
	if cursor < len(m.backlog) {
		return m.backlog[cursor]
	}
	return nil
}

// HasActiveSearch returns false (no search in this view).
func (m *PriorityViewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *PriorityViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *PriorityViewModel) Init() tea.Cmd {
	return nil
}

func (m *PriorityViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeypress(msg)
	}

	return m, nil
}

func (m *PriorityViewModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return ClosePriorityViewMsg{}
		}

	case "enter":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: pt.Task}
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

	case "g":
		m.cursor = 0
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

	// Toggle waiting on section
	case "tab":
		m.waitingOnCollapsed = !m.waitingOnCollapsed
		return m, nil

	// Status changes
	case "s": // Start (in_progress)
		return m.changeTaskStatus(models.StatusInProgress)

	case "m": // Mark complete (done)
		return m.changeTaskStatus(models.StatusDone)

	case "b": // Block
		return m.changeTaskStatus(models.StatusBlocked)

	case "o": // Reopen (todo)
		return m.changeTaskStatus(models.StatusTodo)

	case "x": // Cancel
		return m.changeTaskStatus(models.StatusCancelled)

	// Priority changes
	case "1":
		return m.changeTaskPriority(1)
	case "2":
		return m.changeTaskPriority(2)
	case "3":
		return m.changeTaskPriority(3)
	case "4":
		return m.changeTaskPriority(4)
	case "5":
		return m.changeTaskPriority(5)

	case "e":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewPriorityView,
				ViewName:   "Priority View",
				Items:      m.GetHelpItems(),
			}
		}

	case "r":
		storage.LoadProject(m.project)
		m.loadActionableTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// changeTaskStatus changes the status of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	task := pt.Task
	if task.Status == newStatus {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
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
	if err := storage.SaveProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	// Reload and refresh
	storage.LoadProject(m.project)
	m.loadActionableTasks()
	m.clampCursor()

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor and refreshes.
func (m *PriorityViewModel) changeTaskPriority(newPriority int) (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}

	task := pt.Task
	if task.Priority == newPriority {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
	}

	// Update task priority
	task.Priority = newPriority
	task.UpdatedAt = time.Now()

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	// Reload and refresh
	storage.LoadProject(m.project)
	m.loadActionableTasks()
	m.clampCursor()

	return m, nil
}

// clampCursor ensures cursor is within valid range.
func (m *PriorityViewModel) clampCursor() {
	if m.cursor >= m.totalTasks {
		m.cursor = m.totalTasks - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *PriorityViewModel) View() string {
	var b strings.Builder

	// Title
	title := PriorityViewTitleStyle.Render("Priority View")
	projectName := MutedStyle.Render(m.project.Name)
	b.WriteString(title + " " + projectName)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(m.width-2, 60)))
	b.WriteString("\n\n")

	if m.totalTasks == 0 && len(m.waitingOn) == 0 {
		b.WriteString(MutedStyle.Render("No actionable tasks."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close"))
		return b.String()
	}

	// Calculate available height for task list
	contentHeight := m.height - 8

	// Render groups with scroll awareness
	currentIndex := 0
	lines := m.renderAllGroups(&currentIndex)

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

	// Footer
	footer := HelpStyle.Render("↑/↓ navigate • Enter view • e edit • s start • m done • 1-5 priority • ? help • q close")
	b.WriteString(footer)

	return b.String()
}

// renderAllGroups renders all task groups and returns the lines.
func (m *PriorityViewModel) renderAllGroups(currentIndex *int) []string {
	var lines []string

	if len(m.overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("OVERDUE", len(m.overdue), true))
		lines = append(lines, m.renderGroup(m.overdue, currentIndex, true)...)
		lines = append(lines, "")
	}

	if len(m.today) > 0 {
		lines = append(lines, m.renderGroupHeader("DUE TODAY", len(m.today), false))
		lines = append(lines, m.renderGroup(m.today, currentIndex, false)...)
		lines = append(lines, "")
	}

	if len(m.thisWeek) > 0 {
		lines = append(lines, m.renderGroupHeader("THIS WEEK", len(m.thisWeek), false))
		lines = append(lines, m.renderGroup(m.thisWeek, currentIndex, false)...)
		lines = append(lines, "")
	}

	if len(m.later) > 0 {
		lines = append(lines, m.renderGroupHeader("LATER", len(m.later), false))
		lines = append(lines, m.renderGroup(m.later, currentIndex, false)...)
		lines = append(lines, "")
	}

	if len(m.backlog) > 0 {
		lines = append(lines, m.renderGroupHeader("BACKLOG", len(m.backlog), false))
		lines = append(lines, m.renderGroup(m.backlog, currentIndex, false)...)
		lines = append(lines, "")
	}

	// Waiting On section (blocked tasks)
	if len(m.waitingOn) > 0 {
		lines = append(lines, m.renderWaitingOnHeader())
		if !m.waitingOnCollapsed {
			// Waiting on tasks are not navigable
			for _, pt := range m.waitingOn {
				lines = append(lines, m.renderWaitingOnTask(pt))
			}
		}
	}

	return lines
}

// renderGroupHeader renders a group header with count.
func (m *PriorityViewModel) renderGroupHeader(name string, count int, isOverdue bool) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	if isOverdue {
		return OverdueStyle.Bold(true).Render(header)
	}
	return PriorityGroupHeaderStyle.Render(header)
}

// renderWaitingOnHeader renders the collapsible waiting on section header.
func (m *PriorityViewModel) renderWaitingOnHeader() string {
	indicator := "▶"
	if !m.waitingOnCollapsed {
		indicator = "▼"
	}
	header := fmt.Sprintf("%s Waiting On (%d blocked)", indicator, len(m.waitingOn))
	return PriorityWaitingHeaderStyle.Render(header)
}

// renderGroup renders a group of tasks.
func (m *PriorityViewModel) renderGroup(tasks []*PriorityTask, currentIndex *int, isOverdue bool) []string {
	var lines []string
	maxTitleLen := m.width - 45 // Account for other elements

	for _, pt := range tasks {
		isSelected := *currentIndex == m.cursor
		line := m.renderTaskLine(pt, isOverdue, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single actionable task line.
func (m *PriorityViewModel) renderTaskLine(pt *PriorityTask, isOverdue bool, isSelected bool, maxTitleLen int) string {
	task := pt.Task

	// Cursor indicator
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Status icon with style
	statusStyle := GetStatusStyle(string(task.Status))
	statusIcon := statusStyle.Render(task.Status.Icon())

	// Type icon with style
	typeStyle := GetTypeStyle(string(task.Type))
	typeIcon := typeStyle.Render(task.Type.Icon())

	// Priority badge
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Tags
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
	if pt.RelativeDue != "" {
		if isOverdue {
			dueStr = OverdueStyle.Render(pt.RelativeDue)
		} else if pt.RelativeDue == "today" {
			dueStr = DueTodayStyle.Render(pt.RelativeDue)
		} else {
			dueStr = DueDateStyle.Render(pt.RelativeDue)
		}
	}

	// Build line
	line := fmt.Sprintf("%s%s %s %s %s%s", cursor, statusIcon, typeIcon, priorityBadge, title, tagsStr)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderWaitingOnTask renders a blocked task in the waiting on section.
func (m *PriorityViewModel) renderWaitingOnTask(pt *PriorityTask) string {
	task := pt.Task
	maxTitleLen := m.width - 50

	// Blocked status icon
	statusIcon := BlockedStyle.Render("◈")

	// Type icon
	typeStyle := GetTypeStyle(string(task.Type))
	typeIcon := typeStyle.Render(task.Type.Icon())

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Blocking reason
	blockingReason := m.getBlockingReason(task)

	return fmt.Sprintf("    %s %s %s %s", statusIcon, typeIcon, title, MutedStyle.Render(blockingReason))
}

// getBlockingReason returns a description of what's blocking a task.
func (m *PriorityViewModel) getBlockingReason(task *models.Task) string {
	if task.Status == models.StatusBlocked {
		return "(blocked)"
	}

	// Find incomplete dependencies
	for _, depID := range task.DependsOn {
		if dep := m.project.GetTaskByID(depID); dep != nil {
			if !dep.Status.IsComplete() {
				return fmt.Sprintf("← %s", util.Truncate(dep.Title, 20))
			}
		}
	}

	return "(blocked)"
}

// renderPriorityBadge renders a priority indicator.
func (m *PriorityViewModel) renderPriorityBadge(priority int) string {
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

// GetHelpItems returns help items for this view.
func (m *PriorityViewModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Category: "Navigation", Key: "↑/k", Description: "Move up"},
		{Category: "Navigation", Key: "↓/j", Description: "Move down"},
		{Category: "Navigation", Key: "g/G", Description: "Jump to top/bottom"},
		{Category: "Navigation", Key: "Ctrl+u/d", Description: "Page up/down"},
		{Category: "Actions", Key: "Enter", Description: "View task details"},
		{Category: "Actions", Key: "e", Description: "Edit task"},
		{Category: "Actions", Key: "s", Description: "Start task (in progress)"},
		{Category: "Actions", Key: "m", Description: "Mark task complete"},
		{Category: "Actions", Key: "b", Description: "Block task"},
		{Category: "Actions", Key: "o", Description: "Reopen task (todo)"},
		{Category: "Actions", Key: "x", Description: "Cancel task"},
		{Category: "Actions", Key: "1-5", Description: "Set priority (1=highest)"},
		{Category: "View", Key: "Tab", Description: "Toggle waiting section"},
		{Category: "View", Key: "r", Description: "Refresh"},
		{Category: "View", Key: "?", Description: "Show help"},
		{Category: "View", Key: "q/Esc", Description: "Close priority view"},
	}
}
