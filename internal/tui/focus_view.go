package tui

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var focusTabNames = []string{"All", "By Project", "By Priority", "Due Soon"}

const (
	focusTabAll        = 0
	focusTabByProject  = 1
	focusTabByPriority = 2
	focusTabDueSoon    = 3
)

// FocusTask wraps a task with its parent project for cross-project display.
type FocusTask struct {
	Task    *models.Task
	Project *models.Project
}

// FocusViewModel displays all active-flagged tasks across projects.
type FocusViewModel struct {
	config         *config.Config
	projectService *ProjectService

	// Tasks
	tasks    []FocusTask
	filtered []FocusTask

	// Tabs
	activeTab int

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

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
		config:         cfg,
		projectService: NewProjectService(),
	}
	m.loadTasks()
	return m
}

// loadTasks aggregates all active-flagged tasks from all active projects.
func (m *FocusViewModel) loadTasks() {
	m.tasks = nil

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
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
			// Skip snoozed tasks
			if task.IsSnoozed() {
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

	m.applyFilter()
}

// applyFilter filters and sorts tasks based on the active tab, then applies sub-tab narrowing.
func (m *FocusViewModel) applyFilter() {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch m.activeTab {
	case focusTabByProject:
		m.filtered = make([]FocusTask, len(m.tasks))
		copy(m.filtered, m.tasks)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Project.Name < m.filtered[j].Project.Name
		})
	case focusTabByPriority:
		m.filtered = make([]FocusTask, len(m.tasks))
		copy(m.filtered, m.tasks)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Task.Priority < m.filtered[j].Task.Priority
		})
	case focusTabDueSoon:
		m.filtered = nil
		sevenDaysLater := today.AddDate(0, 0, 7)
		for _, ft := range m.tasks {
			if ft.Task.DueDate == nil {
				continue
			}
			dueDay := time.Date(ft.Task.DueDate.Year(), ft.Task.DueDate.Month(), ft.Task.DueDate.Day(), 0, 0, 0, 0, now.Location())
			if dueDay.Before(sevenDaysLater) {
				m.filtered = append(m.filtered, ft)
			}
		}
	default:
		m.filtered = make([]FocusTask, len(m.tasks))
		copy(m.filtered, m.tasks)
	}

	// Compute sub-tabs based on the filtered list
	m.computeSubTabs()

	// Apply sub-tab narrowing
	if filter := m.activeSubTabFilter(); filter != "" {
		var narrowed []FocusTask
		for _, ft := range m.filtered {
			switch m.activeTab {
			case focusTabByProject:
				if ft.Project.Name == filter {
					narrowed = append(narrowed, ft)
				}
			case focusTabByPriority:
				if fmt.Sprintf("P%d", ft.Task.Priority) == filter {
					narrowed = append(narrowed, ft)
				}
			case focusTabDueSoon:
				bucket := m.dueBucket(ft.Task, now, today)
				if bucket == filter {
					narrowed = append(narrowed, ft)
				}
			}
		}
		m.filtered = narrowed
	}
}

// computeSubTabs builds the dynamic sub-tab names based on active tab and filtered tasks.
func (m *FocusViewModel) computeSubTabs() {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch m.activeTab {
	case focusTabByProject:
		seen := map[string]bool{}
		var names []string
		for _, ft := range m.filtered {
			if !seen[ft.Project.Name] {
				seen[ft.Project.Name] = true
				names = append(names, ft.Project.Name)
			}
		}
		sort.Strings(names)
		m.subTabNames = names

	case focusTabByPriority:
		seen := map[int]bool{}
		var priorities []int
		for _, ft := range m.filtered {
			if !seen[ft.Task.Priority] {
				seen[ft.Task.Priority] = true
				priorities = append(priorities, ft.Task.Priority)
			}
		}
		sort.Ints(priorities)
		m.subTabNames = nil
		for _, p := range priorities {
			m.subTabNames = append(m.subTabNames, fmt.Sprintf("P%d", p))
		}

	case focusTabDueSoon:
		bucketOrder := []string{"Today", "Tomorrow", "This Week", "Later"}
		seen := map[string]bool{}
		for _, ft := range m.filtered {
			bucket := m.dueBucket(ft.Task, now, today)
			seen[bucket] = true
		}
		m.subTabNames = nil
		for _, b := range bucketOrder {
			if seen[b] {
				m.subTabNames = append(m.subTabNames, b)
			}
		}

	default:
		m.subTabNames = nil
	}

	// Clamp sub-tab index (0=All, 1..N map to subTabNames[0..N-1])
	if m.subTab > len(m.subTabNames) {
		m.subTab = 0
	}
}

// activeSubTabFilter returns the selected sub-tab value, or "" if no sub-tab filtering.
func (m *FocusViewModel) activeSubTabFilter() string {
	if len(m.subTabNames) <= 1 || m.subTab <= 0 || m.subTab > len(m.subTabNames) {
		return ""
	}
	return m.subTabNames[m.subTab-1]
}

// dueBucket categorizes a task into a due date bucket.
func (m *FocusViewModel) dueBucket(task *models.Task, now, today time.Time) string {
	if task.DueDate == nil {
		return "Later"
	}
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
	daysUntil := int(dueDay.Sub(today).Hours() / 24)
	switch {
	case daysUntil <= 0:
		return "Today"
	case daysUntil == 1:
		return "Tomorrow"
	case daysUntil <= 7:
		return "This Week"
	default:
		return "Later"
	}
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *FocusViewModel) getTaskAtCursor() *FocusTask {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
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
	case "c":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "c"
		}
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
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
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
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	// Tab row focus switching
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil

	// Tab navigation within focused row
	case "[", "h":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			// Cycle sub-tabs (0=All, 1..N=specific)
			total := len(m.subTabNames) + 1 // +1 for "All"
			if m.subTab > 0 {
				m.subTab--
			} else {
				m.subTab = total - 1
			}
			m.applyFilter()
			m.clampCursor()
		} else {
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(focusTabNames) - 1
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.applyFilter()
			m.clampCursor()
		}
		return m, nil

	case "]", "l":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab < total-1 {
				m.subTab++
			} else {
				m.subTab = 0
			}
			m.applyFilter()
			m.clampCursor()
		} else {
			if m.activeTab < len(focusTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.applyFilter()
			m.clampCursor()
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
			return m.changeTaskStatus(models.StatusCanceled)
		case "o":
			return m.changeTaskStatus(models.StatusTodo)
		case "r":
			return m.changeTaskStatus(models.StatusReview)
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
					return SetDueDateMsg{Task: ft.Task, Project: ft.Project}
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

	case "c":
		// Create linked task or navigate
		ft := m.getTaskAtCursor()
		if ft == nil {
			return m, nil
		}
		task := ft.Task
		project := ft.Project

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

// toggleActive removes the task from focus by clearing its Active flag.
func (m *FocusViewModel) toggleActive() (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	task := ft.Task
	project := ft.Project

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Toggle active flag off
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Active = false
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload and refresh view
	_ = m.projectService.Load(project)
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
	case models.StatusCanceled:
		if !task.Status.IsActive() {
			m.statusMessage = "Task is not active"
			return m, nil
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			m.statusMessage = "Task is already todo"
			return m, nil
		}
	case models.StatusReview:
		if !task.Status.IsActive() || task.Status == models.StatusReview {
			m.statusMessage = "Task must be active and not already in review"
			return m, nil
		}
	}

	// Backup before change
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task status
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Status = newStatus
		if newStatus.IsComplete() {
			now := time.Now()
			t.CompletedAt = &now
			// Clear active flag when completing
			t.Active = false
		} else {
			t.CompletedAt = nil
		}
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
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
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Update task priority
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
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
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Set due date
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = &newDate
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
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
	if err := m.projectService.Backup(project); err != nil {
		m.err = fmt.Errorf("backup failed: %w", err)
		return m, nil
	}

	// Clear due date
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = nil
	})

	// Save project
	if err := m.projectService.Save(project); err != nil {
		m.err = fmt.Errorf("save failed: %w", err)
		return m, nil
	}

	// Reload project and refresh view
	_ = m.projectService.Load(project)
	m.loadTasks()
	m.clampCursor()

	m.statusMessage = "Due date cleared"

	return m, func() tea.Msg {
		return TaskUpdatedMsg{Task: task}
	}
}

// clampCursor ensures cursor is within valid range.
func (m *FocusViewModel) clampCursor() {
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// HasPendingPrefix returns true if waiting for second key in prefix sequence.
func (m *FocusViewModel) HasPendingPrefix() bool {
	return m.pendingPrefix != ""
}

// HasTextInput returns false for the focus view.
func (m *FocusViewModel) HasTextInput() bool {
	return false
}

// ShouldCaptureEsc returns true when prefix mode is active.
func (m *FocusViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix()
}

// GetHelpItems returns help items for this view.
func (m *FocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusView)
}
