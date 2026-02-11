package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// ProjectFocusViewModel displays active-flagged tasks for a single project.
type ProjectFocusViewModel struct {
	project *models.Project

	// Tasks
	tasks []*models.Task

	// Navigation
	cursor int

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string

	// UI feedback
	statusMessage string
	err           error

	// Dimensions
	width  int
	height int
}

// NewProjectFocusViewModel creates a new project focus view model.
func NewProjectFocusViewModel(project *models.Project) *ProjectFocusViewModel {
	m := &ProjectFocusViewModel{
		project: project,
	}
	m.loadTasks()
	return m
}

// loadTasks collects active-flagged tasks from the project.
func (m *ProjectFocusViewModel) loadTasks() {
	m.tasks = nil

	for _, task := range m.project.Tasks {
		if !task.Active {
			continue
		}
		// Skip completed tasks
		if task.Status.IsComplete() {
			continue
		}
		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}
		m.tasks = append(m.tasks, task)
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Priority != m.tasks[j].Priority {
			return m.tasks[i].Priority < m.tasks[j].Priority
		}
		return m.tasks[i].UpdatedAt.After(m.tasks[j].UpdatedAt)
	})

	m.computeSubTabs()
}

// computeSubTabs builds dynamic sub-tab names from unique CustomSection values.
func (m *ProjectFocusViewModel) computeSubTabs() {
	seen := map[string]bool{}
	var names []string
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if !seen[section] {
			seen[section] = true
			names = append(names, section)
		}
	}
	sort.Strings(names)
	m.subTabNames = names

	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *ProjectFocusViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// filteredTasks returns tasks filtered by the active sub-tab.
func (m *ProjectFocusViewModel) filteredTasks() []*models.Task {
	filter := m.activeSubTabFilter()
	if filter == "" {
		return m.tasks
	}
	var result []*models.Task
	for _, task := range m.tasks {
		section := task.CustomSection
		if section == "" {
			section = "(none)"
		}
		if section == filter {
			result = append(result, task)
		}
	}
	return result
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *ProjectFocusViewModel) getTaskAtCursor() *models.Task {
	tasks := m.filteredTasks()
	if m.cursor < 0 || m.cursor >= len(tasks) {
		return nil
	}
	return tasks[m.cursor]
}

// HasActiveSearch returns false (no search in this view).
func (m *ProjectFocusViewModel) HasActiveSearch() bool {
	return false
}

// SetSize sets the view dimensions.
func (m *ProjectFocusViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ProjectFocusViewModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectFocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case TaskUpdatedMsg:
		// Auto-refresh when a task is updated (e.g., after editing)
		_ = storage.LoadProject(m.project)
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *ProjectFocusViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	// Sub-tab navigation
	switch key {
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil
	case "[", "h":
		if len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab > 0 {
				m.subTab--
			} else {
				m.subTab = total - 1
			}
			m.clampCursor()
			return m, nil
		}
	case "]", "l":
		if len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab < total-1 {
				m.subTab++
			} else {
				m.subTab = 0
			}
			m.clampCursor()
			return m, nil
		}
	}

	tasks := m.filteredTasks()

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	case "enter":
		// Close and return to project tree (task is already in this project)
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	// Navigation
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(tasks)-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if len(tasks) > 0 {
			m.cursor = len(tasks) - 1
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
		if m.cursor >= len(tasks) {
			m.cursor = len(tasks) - 1
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
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}
		}
		return m, nil

	// Refresh
	case "r":
		_ = storage.LoadProject(m.project)
		m.loadTasks()
		m.clampCursor()
		m.statusMessage = "Refreshed"
		return m, nil

	// Copy
	case "y":
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return m, nil

	case "Y":
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return m, nil

	case "ctrl+y":
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: task}
			}
		}
		return m, nil

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewProjectFocusView,
				ViewName:   "Project Focus View",
				Items:      m.GetHelpItems(),
			}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectFocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
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
			if task := m.getTaskAtCursor(); task != nil {
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: m.project}
				}
			}
		}

	case "g":
		switch key {
		case "g":
			// gg: Go to first item
			m.cursor = 0
			return m, nil
		}
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// toggleActive removes the task from focus by clearing its Active flag.
func (m *ProjectFocusViewModel) toggleActive() (tea.Model, tea.Cmd) {
	task := m.getTaskAtCursor()
	if task == nil {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Toggle active flag off
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Active = false
	})

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload and refresh view
	_ = storage.LoadProject(m.project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Removed #%d from focus", task.ShortID)

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// changeTaskStatus changes the status of the task at cursor.
func (m *ProjectFocusViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	task := m.getTaskAtCursor()
	if task == nil {
		return m, nil
	}

	if msg := task.CanTransitionTo(newStatus); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	// Project-level check: incomplete dependencies block completion
	if newStatus == models.StatusDone {
		if m.project.HasIncompleteDependencies(task) {
			m.err = fmt.Errorf("cannot complete: has incomplete dependencies")
			return m, nil
		}
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Apply via lifecycle method
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(newStatus)
	})

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(m.project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus)

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *ProjectFocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	task := m.getTaskAtCursor()
	if task == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)

	if task.Priority == priority {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task priority
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(m.project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *ProjectFocusViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	task := m.getTaskAtCursor()
	if task == nil {
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Set due date
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = util.StartOfDay(newDate)
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = &newDate
	})

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(m.project)
	m.loadTasks()
	m.clampCursor()

	label := "today"
	switch daysFromNow {
	case 1:
		label = "tomorrow"
	case 7:
		label = "next week"
	case 30:
		label = "next month"
	}
	m.statusMessage = fmt.Sprintf("Due date set to %s", label)

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// clearDueDate clears the due date from the task at cursor.
func (m *ProjectFocusViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	task := m.getTaskAtCursor()
	if task == nil {
		return m, nil
	}

	if task.DueDate == nil {
		m.statusMessage = "Task has no due date"
		return m, nil
	}

	// Backup before change
	if err := storage.BackupProject(m.project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Clear due date
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = nil
	})

	// Save project
	if err := storage.SaveProject(m.project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = storage.LoadProject(m.project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// clampCursor ensures cursor is within valid range.
func (m *ProjectFocusViewModel) clampCursor() {
	tasks := m.filteredTasks()
	if m.cursor >= len(tasks) {
		m.cursor = len(tasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *ProjectFocusViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// GetHelpItems returns help items for this view.
func (m *ProjectFocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewProjectFocusView)
}

func (m *ProjectFocusViewModel) View() string {
	tasks := m.filteredTasks()

	var b strings.Builder

	subtitle := fmt.Sprintf("%d active tasks", len(m.tasks))
	if len(m.tasks) == 0 {
		subtitle = "no active tasks"
	}
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Focus: %s", m.project.Name), subtitle))
	b.WriteString("\n\n")

	if len(m.tasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks in focus."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Mark tasks as active with * in the project tree."))
		b.WriteString("\n")
		left := HelpStyle.Render("q return")
		b.WriteString(renderStatusBarFooter(left, "", m.width))
		return b.String()
	}

	// Calculate available height for task list
	contentHeight := m.height - 8

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
		contentHeight -= 2
	}

	// Render task lines
	var lines []string
	maxTitleLen := m.width - 35

	for i, task := range tasks {
		isSelected := i == m.cursor
		line := m.renderTaskLine(task, isSelected, maxTitleLen)
		lines = append(lines, line)
	}

	if len(tasks) == 0 {
		b.WriteString(MutedStyle.Render("No tasks match this filter."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
		b.WriteString("\n\n")
	}

	b.WriteString(renderStatusMessages(m.statusMessage, m.err))

	// Footer
	footerParts := "j/k nav | * unfocus | ,s/d/b/x status | 1-5 priority | dt/dm due"
	if len(m.subTabNames) > 1 {
		footerParts += " | [/] sections"
	}
	footerParts += " | ? help"
	left := HelpStyle.Render(footerParts)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(tasks)))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *ProjectFocusViewModel) renderSubTabBar() string {
	if len(m.subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(m.subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, m.subTabNames...)

	bar := RenderTabBar(tabs, m.subTab, min(m.width-6, 54)-2, 1)

	focusIndicator := lipgloss.NewStyle().Foreground(ColorPrimary).Render("▐")
	unfocusedPad := " "
	if m.subTabRowFocused {
		bar = focusIndicator + bar
	} else {
		bar = unfocusedPad + bar
	}
	return bar
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *ProjectFocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",": StatusPrefixHints,
			"d": DueDatePrefixHints,
			"g": ProjectFocusNavigationPrefixHints,
		},
	}
}

// ProjectFocusNavigationPrefixHints contains "g" prefix hints for project focus view.
var ProjectFocusNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
}

// renderTaskLine renders a single task line.
func (m *ProjectFocusViewModel) renderTaskLine(task *models.Task, isSelected bool, maxTitleLen int) string {
	// Cursor
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Status icon
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	// Type icon
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Priority badge
	priorityBadge := renderPriorityBadge(task.Priority)

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Custom section badge
	csStr := ""
	if task.CustomSection != "" {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csStr = " " + csStyle.Render("@ "+task.CustomSection)
	}

	tagsStr := renderInlineTags(task.Tags)

	// Due date
	dueStr := ""
	if task.DueDate != nil {
		dueStr = formatRelativeDue(task.DueDate, true)
	}

	// Build line (no project name needed)
	line := fmt.Sprintf("%s%s %s %s %s%s%s", cursor, statusIcon, typeIcon, priorityBadge, title, csStr, tagsStr)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}
