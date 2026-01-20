package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// DueDateTask wraps a task with its parent project for cross-project display.
type DueDateTask struct {
	Task    *models.Task
	Project *models.Project
}

// DueDateGroup represents a category of tasks by due date urgency.
type DueDateGroup int

const (
	GroupOverdue DueDateGroup = iota
	GroupToday
	GroupThisWeek
	GroupLater
)

// DueDateOverviewModel displays all tasks with due dates across all projects.
type DueDateOverviewModel struct {
	config *config.Config

	// Grouped tasks
	overdue  []DueDateTask
	today    []DueDateTask
	thisWeek []DueDateTask
	later    []DueDateTask

	// Navigation
	cursor      int // Absolute position across all groups
	totalTasks  int
	groupStarts []int // Starting index of each group in flattened view

	// Dimensions
	width  int
	height int
}

// NewDueDateOverviewModel creates a new due date overview model.
func NewDueDateOverviewModel(cfg *config.Config) *DueDateOverviewModel {
	m := &DueDateOverviewModel{
		config: cfg,
	}
	m.loadTasksWithDueDates()
	return m
}

// loadTasksWithDueDates aggregates tasks with due dates from all active projects.
func (m *DueDateOverviewModel) loadTasksWithDueDates() {
	m.overdue = nil
	m.today = nil
	m.thisWeek = nil
	m.later = nil

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if task.DueDate == nil || task.Status.IsComplete() {
				continue
			}

			dt := DueDateTask{Task: task, Project: project}
			switch m.categorizeTask(task) {
			case GroupOverdue:
				m.overdue = append(m.overdue, dt)
			case GroupToday:
				m.today = append(m.today, dt)
			case GroupThisWeek:
				m.thisWeek = append(m.thisWeek, dt)
			case GroupLater:
				m.later = append(m.later, dt)
			}
		}
	}

	// Sort each group by due date (earliest first)
	sortByDueDate := func(tasks []DueDateTask) {
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Task.DueDate.Before(*tasks[j].Task.DueDate)
		})
	}
	sortByDueDate(m.overdue)
	sortByDueDate(m.today)
	sortByDueDate(m.thisWeek)
	sortByDueDate(m.later)

	// Calculate navigation indices
	m.totalTasks = len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later)
	m.groupStarts = []int{
		0,
		len(m.overdue),
		len(m.overdue) + len(m.today),
		len(m.overdue) + len(m.today) + len(m.thisWeek),
	}
}

// categorizeTask determines which urgency group a task belongs to.
func (m *DueDateOverviewModel) categorizeTask(task *models.Task) DueDateGroup {
	if task.DueDate == nil {
		return GroupLater
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		return GroupOverdue
	}
	if dueDay.Equal(today) {
		return GroupToday
	}
	weekEnd := today.AddDate(0, 0, 7)
	if dueDay.Before(weekEnd) {
		return GroupThisWeek
	}
	return GroupLater
}

// getTaskAtCursor returns the task and project at the current cursor position.
func (m *DueDateOverviewModel) getTaskAtCursor() *DueDateTask {
	if m.totalTasks == 0 {
		return nil
	}

	cursor := m.cursor
	if cursor < m.groupStarts[1] {
		return &m.overdue[cursor]
	}
	cursor -= m.groupStarts[1]
	if cursor < len(m.today) {
		return &m.today[cursor]
	}
	cursor -= len(m.today)
	if cursor < len(m.thisWeek) {
		return &m.thisWeek[cursor]
	}
	cursor -= len(m.thisWeek)
	if cursor < len(m.later) {
		return &m.later[cursor]
	}
	return nil
}

// HasActiveSearch returns false (no search in this view).
func (m *DueDateOverviewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *DueDateOverviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *DueDateOverviewModel) Init() tea.Cmd {
	return nil
}

func (m *DueDateOverviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return CloseDueDateOverviewMsg{}
			}
		case "enter":
			if dt := m.getTaskAtCursor(); dt != nil {
				return m, func() tea.Msg {
					return DueDateOverviewSelectMsg{Task: dt.Task, Project: dt.Project}
				}
			}
			return m, nil
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
		}
	}

	return m, nil
}

func (m *DueDateOverviewModel) View() string {
	var b strings.Builder

	if m.totalTasks == 0 {
		b.WriteString(MutedStyle.Render("No tasks with due dates found."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close"))

		return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Due Dates",
			Subtitle: "across all projects",
			Centered: true,
		})
	}

	// Calculate available height for task list (account for card border/padding)
	contentHeight := m.height - 12

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
	footer := HelpStyle.Render("↑/↓/j/k navigate • Enter view task • g/G top/bottom • Esc close")
	b.WriteString(footer)

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Due Dates",
		Subtitle: fmt.Sprintf("%d tasks across all projects", m.totalTasks),
		Centered: true,
	})
}

// renderAllGroups renders all task groups and returns the lines.
func (m *DueDateOverviewModel) renderAllGroups(currentIndex *int) []string {
	var lines []string
	hasContent := false
	separatorWidth := min(m.width-10, 50)

	if len(m.overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("Overdue", len(m.overdue), true))
		lines = append(lines, m.renderGroup(m.overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(m.today) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("Today", len(m.today), false))
		lines = append(lines, m.renderGroup(m.today, currentIndex, false)...)
		hasContent = true
	}

	if len(m.thisWeek) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("This Week", len(m.thisWeek), false))
		lines = append(lines, m.renderGroup(m.thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(m.later) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("Later", len(m.later), false))
		lines = append(lines, m.renderGroup(m.later, currentIndex, false)...)
	}

	return lines
}

// renderGroupHeader renders a group header with count.
func (m *DueDateOverviewModel) renderGroupHeader(name string, count int, isOverdue bool) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	if isOverdue {
		return OverdueStyle.Bold(true).Render(header)
	}
	return DueDateGroupHeaderStyle.Render(header)
}

// renderGroup renders a group of tasks.
func (m *DueDateOverviewModel) renderGroup(tasks []DueDateTask, currentIndex *int, isOverdue bool) []string {
	var lines []string
	maxTitleLen := m.width - 35 // Account for date, project, icons

	for _, dt := range tasks {
		isSelected := *currentIndex == m.cursor
		line := m.renderTaskLine(dt, isOverdue, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single task line.
func (m *DueDateOverviewModel) renderTaskLine(dt DueDateTask, isOverdue bool, isSelected bool, maxTitleLen int) string {
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Date indicator and formatted date
	dateStr := m.formatDueDate(dt.Task, isOverdue)

	// Project name in brackets
	projectName := util.Truncate(dt.Project.Name, 15)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	// Task type icon
	typeStyle := GetTypeStyle(string(dt.Task.Type))
	typeIcon := typeStyle.Render(dt.Task.Type.Icon())

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(dt.Task.Title, maxTitleLen)

	line := fmt.Sprintf("%s%s %s %s %s", cursor, dateStr, projectStr, typeIcon, title)

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// formatDueDate formats the due date for display.
func (m *DueDateOverviewModel) formatDueDate(task *models.Task, isOverdue bool) string {
	if task.DueDate == nil {
		return "         "
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	var dateStr string
	var style lipgloss.Style

	if isOverdue {
		dateStr = task.DueDate.Format("Jan 2")
		style = OverdueStyle
	} else if dueDay.Equal(today) {
		dateStr = "Today"
		style = DueTodayStyle
	} else if dueDay.Equal(today.AddDate(0, 0, 1)) {
		dateStr = "Tomorrow"
		style = DueDateStyle
	} else if task.DueDate.Year() != now.Year() {
		dateStr = task.DueDate.Format("Jan 2 '06")
		style = DueDateStyle
	} else {
		dateStr = task.DueDate.Format("Jan 2")
		style = DueDateStyle
	}

	// Pad to consistent width
	return style.Render(fmt.Sprintf("%-9s", dateStr))
}

// CloseDueDateOverviewMsg is sent when the overview should close.
type CloseDueDateOverviewMsg struct{}

// DueDateOverviewSelectMsg is sent when a task is selected.
type DueDateOverviewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowDueDateOverviewMsg is sent to open the due date overview.
type ShowDueDateOverviewMsg struct{}
