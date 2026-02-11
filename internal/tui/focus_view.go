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

	// Filter
	filter CommonFilterState

	// Navigation
	cursor int

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

// NewFocusViewModel creates a new focus view model.
func NewFocusViewModel(cfg *config.Config) *FocusViewModel {
	m := &FocusViewModel{
		config:         cfg,
		projectService: NewProjectService(),
		filter: CommonFilterState{
			Tags:  make(map[string]bool),
			Attrs: make(map[string]string),
		},
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

	// Apply common filters first (type, status, priority, tags, attrs, score)
	baseTasks := m.tasks
	if hasActiveCommonFilters(&m.filter) {
		baseTasks = nil
		for _, ft := range m.tasks {
			// Use empty projectID since focus view aggregates across projects
			if passesCommonFilters(ft.Task, &m.filter, ft.Project.ID, m.config) {
				baseTasks = append(baseTasks, ft)
			}
		}
	}

	switch m.activeTab {
	case focusTabByProject:
		m.filtered = make([]FocusTask, len(baseTasks))
		copy(m.filtered, baseTasks)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Project.Name < m.filtered[j].Project.Name
		})
	case focusTabByPriority:
		m.filtered = make([]FocusTask, len(baseTasks))
		copy(m.filtered, baseTasks)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Task.Priority < m.filtered[j].Task.Priority
		})
	case focusTabDueSoon:
		m.filtered = nil
		sevenDaysLater := today.AddDate(0, 0, 7)
		for _, ft := range baseTasks {
			if ft.Task.DueDate == nil {
				continue
			}
			dueDay := time.Date(ft.Task.DueDate.Year(), ft.Task.DueDate.Month(), ft.Task.DueDate.Day(), 0, 0, 0, 0, now.Location())
			if dueDay.Before(sevenDaysLater) {
				m.filtered = append(m.filtered, ft)
			}
		}
	default:
		m.filtered = make([]FocusTask, len(baseTasks))
		copy(m.filtered, baseTasks)
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
func (m *FocusViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.statusMessage = ""

	// Handle confirm dialog
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

	// Activate prefix mode
	switch key {
	case ",", "d", "g":
		m.pendingPrefix = key
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	case "m", "c":
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = key
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
			return m, nil
		}
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

	// Tags
	case "t":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: ft.Task, Project: ft.Project, CurrentTags: append([]string{}, ft.Task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: ft.Task, Project: ft.Project}
			}
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if ft := m.getTaskAtCursor(); ft != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", ft.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
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
	case "F":
		return m.handleFocusFilterPrefix(key)
	case "Ft", "Fs", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleFocusFilterSubPrefix(prefix, key)

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

	case "m":
		section, ok := sectionForMoveKey(key)
		if !ok {
			return m, nil
		}
		return m.sectionMove(section)

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

	result := applyRemoveFromFocus(m.projectService, ft.Project, ft.Task.ID, ft.Task.ShortID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// changeTaskStatus changes the status of the task at cursor.
func (m *FocusViewModel) changeTaskStatus(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	result := applyStatusChange(m.projectService, ft.Project, ft.Task.ID, newStatus)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// changeTaskPriority changes the priority of the task at cursor.
func (m *FocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	priority, _ := strconv.Atoi(key)
	result := applyPriorityChange(m.projectService, ft.Project, ft.Task.ID, priority)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// setQuickDueDate sets the due date to a relative number of days from today.
func (m *FocusViewModel) setQuickDueDate(daysFromNow int) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	result := applyQuickDueDate(m.projectService, ft.Project, ft.Task.ID, daysFromNow)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// clearDueDate clears the due date from the task at cursor.
func (m *FocusViewModel) clearDueDate() (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	result := applyClearDueDate(m.projectService, ft.Project, ft.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// sectionMove moves the task at cursor to the specified GTD section.
func (m *FocusViewModel) sectionMove(section string) (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	result := applySectionChange(m.projectService, ft.Project, ft.Task.ID, section)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
}

// deleteTask deletes the task at cursor.
func (m *FocusViewModel) deleteTask() (tea.Model, tea.Cmd) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return m, nil
	}

	result := applyDeleteTask(m.projectService, ft.Project, ft.Task.ID)
	if result.Err != nil {
		m.err = result.Err
	} else if result.StatusMessage != "" {
		m.statusMessage = result.StatusMessage
	}
	m.loadTasks()
	m.clampCursor()
	return m, result.Cmd
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

// ShouldCaptureEsc returns true when prefix mode, filters, or confirm dialog are active.
func (m *FocusViewModel) ShouldCaptureEsc() bool {
	return m.HasPendingPrefix() || m.HasActiveFilters() || m.confirm.IsActive()
}

// GetHelpItems returns help items for this view.
func (m *FocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusView)
}
