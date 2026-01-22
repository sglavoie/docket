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

// SuggestedTask represents the single most actionable task with reasoning.
type SuggestedTask struct {
	Task   *PriorityTask
	Score  int
	Reason string
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

	// Smart suggestion
	suggestion   *SuggestedTask  // The single most actionable task
	dismissedIDs map[string]bool // Task IDs dismissed this session

	// Navigation
	cursor      int   // Absolute position across all visible groups
	totalTasks  int   // Total count of visible actionable tasks
	groupStarts []int // Starting index of each group in flattened view

	// Waiting On section state
	waitingOnCollapsed bool

	// Prefix key state for Vim-style two-key sequences
	pendingPrefix string // "g" when waiting for second key

	// Dimensions
	width  int
	height int
}

// NewPriorityViewModel creates a new priority view for the given project.
func NewPriorityViewModel(project *models.Project) *PriorityViewModel {
	m := &PriorityViewModel{
		project:            project,
		waitingOnCollapsed: true,
		dismissedIDs:       make(map[string]bool),
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

	// Compute the smart suggestion
	m.computeSuggestion()
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

// computeSuggestionScore calculates the suggestion score for a task.
// Builds on urgencyScore with additional factors for actionability.
// Higher scores = more recommended as the suggested next task.
func (m *PriorityViewModel) computeSuggestionScore(task *models.Task) int {
	// Start with base urgency score
	score := m.computeUrgencyScore(task)

	// Leaf task bonus: prefer actionable leaf tasks over containers
	if task.IsLeaf() && !task.IsNote() {
		score += 800
	}

	// Container penalty: penalize tasks with incomplete children
	if len(task.Children) > 0 && !m.allChildrenComplete(task) {
		score -= 1000
	}

	// Task age: older tasks get a small boost to avoid stagnation
	daysSinceCreated := int(time.Since(task.CreatedAt).Hours() / 24)
	ageBonus := daysSinceCreated / 7 // 30 points per week
	if ageBonus > 10 {
		ageBonus = 10 // Cap at 300 points
	}
	score += ageBonus * 30

	// Recent momentum: boost tasks with recent activity
	daysSinceUpdated := int(time.Since(task.UpdatedAt).Hours() / 24)
	if daysSinceUpdated == 0 {
		score += 400 // Updated today
	} else if daysSinceUpdated <= 3 {
		score += 200 // Updated within 3 days
	}

	// Unblocking bonus: prefer tasks that unblock others
	blockedByThisCount := len(m.project.GetBlockedByTasks(task))
	unblockingBonus := blockedByThisCount
	if unblockingBonus > 3 {
		unblockingBonus = 3 // Cap at 3
	}
	score += unblockingBonus * 150

	// Section penalty: deprioritize Backlog and Pending section tasks
	if task.Section == models.SectionBacklog {
		score -= 300
	} else if task.Section == models.SectionPending {
		score -= 200
	}

	return score
}

// allChildrenComplete returns true if all non-note children of the task are complete.
func (m *PriorityViewModel) allChildrenComplete(task *models.Task) bool {
	for _, child := range task.Children {
		if child.IsNote() {
			continue
		}
		if !child.Status.IsComplete() {
			return false
		}
	}
	return true
}

// computeSuggestion finds the highest-scoring non-dismissed actionable task.
func (m *PriorityViewModel) computeSuggestion() {
	m.suggestion = nil

	// Collect all actionable tasks (not blocked, not completed)
	var candidates []*PriorityTask
	candidates = append(candidates, m.overdue...)
	candidates = append(candidates, m.today...)
	candidates = append(candidates, m.thisWeek...)
	candidates = append(candidates, m.later...)
	candidates = append(candidates, m.backlog...)

	if len(candidates) == 0 {
		return
	}

	// Find highest-scoring non-dismissed task
	var best *PriorityTask
	bestScore := -999999

	for _, pt := range candidates {
		if m.dismissedIDs[pt.Task.ID] {
			continue
		}

		suggestionScore := m.computeSuggestionScore(pt.Task)
		if suggestionScore > bestScore {
			bestScore = suggestionScore
			best = pt
		}
	}

	if best == nil {
		return
	}

	m.suggestion = &SuggestedTask{
		Task:   best,
		Score:  bestScore,
		Reason: m.getSuggestionReason(best.Task),
	}
}

// getSuggestionReason generates a human-readable explanation for why a task is suggested.
func (m *PriorityViewModel) getSuggestionReason(task *models.Task) string {
	var reasons []string

	// Priority
	if task.Priority == 1 {
		reasons = append(reasons, "critical priority")
	} else if task.Priority == 2 {
		reasons = append(reasons, "high priority")
	}

	// Due date status
	if task.DueDate != nil {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

		if dueDay.Before(today) {
			reasons = append(reasons, "overdue")
		} else if dueDay.Equal(today) {
			reasons = append(reasons, "due today")
		} else if dueDay.Before(today.AddDate(0, 0, 7)) {
			reasons = append(reasons, "due this week")
		}
	}

	// In-progress status
	if task.Status == models.StatusInProgress {
		reasons = append(reasons, "in progress")
	}

	// Active flag
	if task.Active {
		reasons = append(reasons, "flagged active")
	}

	// Unblocking count
	blockedByThisCount := len(m.project.GetBlockedByTasks(task))
	if blockedByThisCount > 0 {
		if blockedByThisCount == 1 {
			reasons = append(reasons, "unblocks 1 task")
		} else {
			reasons = append(reasons, fmt.Sprintf("unblocks %d tasks", blockedByThisCount))
		}
	}

	// Momentum
	daysSinceUpdated := int(time.Since(task.UpdatedAt).Hours() / 24)
	if daysSinceUpdated == 0 && task.Status != models.StatusInProgress {
		reasons = append(reasons, "recent activity")
	}

	// If no specific reasons, provide a generic one
	if len(reasons) == 0 {
		return "Ready to work on"
	}

	// Join reasons with " + "
	result := ""
	for i, r := range reasons {
		if i > 0 {
			result += " + "
		}
		// Capitalize first reason
		if i == 0 {
			result += strings.ToUpper(r[:1]) + r[1:]
		} else {
			result += r
		}
	}
	return result
}

// dismissSuggestion adds the current suggestion to the dismissed set.
func (m *PriorityViewModel) dismissSuggestion() {
	if m.suggestion == nil {
		return
	}
	m.dismissedIDs[m.suggestion.Task.Task.ID] = true
	m.computeSuggestion()
}

// startSuggestedTask sets the suggested task to in_progress or jumps to it if already started.
func (m *PriorityViewModel) startSuggestedTask() (tea.Model, tea.Cmd) {
	if m.suggestion == nil {
		return m, nil
	}

	task := m.suggestion.Task.Task

	// If already in progress, just jump to it in the list
	if task.Status == models.StatusInProgress {
		// Find the task's position in the list and move cursor there
		m.jumpToTask(task)
		return m, nil
	}

	// Otherwise, start the task
	if err := storage.BackupProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
	}

	task.Start()

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

// jumpToTask moves the cursor to the position of the given task in the list.
func (m *PriorityViewModel) jumpToTask(task *models.Task) {
	index := 0
	for _, pt := range m.overdue {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.today {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.thisWeek {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.later {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
	for _, pt := range m.backlog {
		if pt.Task.ID == task.ID {
			m.cursor = index
			return
		}
		index++
	}
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

	// Handle pending prefix keys (Vim-style gg)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

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

	case ",":
		// Start prefix sequence for status operations
		m.pendingPrefix = ","
		return m, nil

	case "g":
		// Start prefix sequence for gg (Vim-style)
		m.pendingPrefix = "g"
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

	case "y":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "Y":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: pt.Task}
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
		// Refresh: reload project and clear dismissed suggestions
		storage.LoadProject(m.project)
		m.dismissedIDs = make(map[string]bool)
		m.loadActionableTasks()
		m.clampCursor()
		return m, nil

	// Suggestion actions
	case "s":
		// Start suggested task (or jump to it if already in progress)
		return m.startSuggestedTask()

	case "x":
		// Dismiss current suggestion
		m.dismissSuggestion()
		return m, nil
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *PriorityViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
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
		case "o":
			return m.changeTaskStatus(models.StatusTodo)
		case "x":
			return m.changeTaskStatus(models.StatusCancelled)
		}
	case "g":
		switch key {
		case "g":
			// gg: Go to first item
			m.cursor = 0
			return m, nil
		case "t":
			// gt: Go to task in project tree
			if pt := m.getTaskAtCursor(); pt != nil {
				return m, func() tea.Msg {
					return PriorityViewSelectMsg{Task: pt.Task}
				}
			}
			return m, nil
		}
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *PriorityViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
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

	if m.totalTasks == 0 && len(m.waitingOn) == 0 {
		b.WriteString(MutedStyle.Render("No actionable tasks."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close"))

		return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Priority View",
			Subtitle: m.project.Name,
			Centered: true,
		})
	}

	// Calculate available height for task list (account for card border/padding)
	contentHeight := m.height - 12

	// Render suggestion section at top if available
	suggestionSection := m.renderSuggestionSection()
	suggestionLines := 0
	if suggestionSection != "" {
		b.WriteString(suggestionSection)
		b.WriteString("\n\n")
		suggestionLines = 5 // Approximate height of suggestion box
		contentHeight -= suggestionLines
	}

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

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
		b.WriteString("\n")
	}

	// Footer
	footer := HelpStyle.Render("↑/↓ navigate • s start suggested • x dismiss • ,s/,d status • ? help • q close")
	b.WriteString(footer)

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Priority View",
		Subtitle: fmt.Sprintf("%d tasks • %s", m.totalTasks, m.project.Name),
		Centered: true,
	})
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *PriorityViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"g": PriorityViewNavigationPrefixHints,
	}, m.width)
}

// renderSuggestionSection renders the "SUGGESTED NEXT" section at the top.
func (m *PriorityViewModel) renderSuggestionSection() string {
	if m.suggestion == nil {
		return ""
	}

	task := m.suggestion.Task.Task
	boxWidth := min(m.width-6, 54) // Constrain box width

	// Build task line
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Calculate available title width
	// Star + status + type + priority + spacing takes about 15-20 chars
	titleWidth := boxWidth - 22
	if titleWidth < 10 {
		titleWidth = 10
	}
	title := util.Truncate(task.Title, titleWidth)

	// Due date
	dueStr := ""
	if m.suggestion.Task.RelativeDue != "" {
		dueStr = DueDateStyle.Render(m.suggestion.Task.RelativeDue)
	}

	// Star indicator
	star := SuggestionStarStyle.Render("★")

	// Build the task line
	taskLine := fmt.Sprintf("%s %s %s %s %s", star, statusIcon, typeIcon, priorityBadge, title)
	if dueStr != "" {
		taskLine = fmt.Sprintf("%-*s %s", boxWidth-12, taskLine, dueStr)
	}

	// Reason line (muted, indented)
	reasonLine := SuggestionReasonStyle.Render("  " + m.suggestion.Reason)

	// Build the box
	content := taskLine + "\n" + reasonLine
	box := SuggestionBoxStyle.Width(boxWidth).Render(content)

	// Add header
	header := SuggestionHeaderStyle.Render("─ SUGGESTED NEXT ")
	headerPadding := boxWidth - 17
	if headerPadding > 0 {
		header += MutedStyle.Render(strings.Repeat("─", headerPadding))
	}

	return header + "\n" + box
}

// renderAllGroups renders all task groups and returns the lines.
func (m *PriorityViewModel) renderAllGroups(currentIndex *int) []string {
	var lines []string
	hasContent := false
	separatorWidth := min(m.width-10, 50)

	if len(m.overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("OVERDUE", len(m.overdue), true))
		lines = append(lines, m.renderGroup(m.overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(m.today) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("DUE TODAY", len(m.today), false))
		lines = append(lines, m.renderGroup(m.today, currentIndex, false)...)
		hasContent = true
	}

	if len(m.thisWeek) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("THIS WEEK", len(m.thisWeek), false))
		lines = append(lines, m.renderGroup(m.thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(m.later) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("LATER", len(m.later), false))
		lines = append(lines, m.renderGroup(m.later, currentIndex, false)...)
		hasContent = true
	}

	if len(m.backlog) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroup(m.backlog, currentIndex, false)...)
		hasContent = true
	}

	// Waiting On section (blocked tasks)
	if len(m.waitingOn) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
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

	// Star indicator for suggested task
	isSuggested := m.suggestion != nil && m.suggestion.Task.Task.ID == task.ID
	starIndicator := "  "
	if isSuggested {
		starIndicator = SuggestionStarStyle.Render("★ ")
	}

	// Cursor indicator
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Status icon with badge style
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Priority badge
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Title (reduce width if showing star indicator)
	adjustedMaxTitleLen := maxTitleLen
	if isSuggested {
		adjustedMaxTitleLen -= 2 // Account for star indicator
	}
	if adjustedMaxTitleLen < 10 {
		adjustedMaxTitleLen = 10
	}
	title := util.Truncate(task.Title, adjustedMaxTitleLen)

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

	// Build line with star indicator before cursor
	line := fmt.Sprintf("%s%s%s %s %s %s%s", starIndicator, cursor, statusIcon, typeIcon, priorityBadge, title, tagsStr)
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

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

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
	return GetKeybindingRegistry().GetForView(ViewPriorityView)
}
