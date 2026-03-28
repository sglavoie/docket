package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

var focusTabNames = []string{"Queue", "Suggested", "By Project", "By Due Date"}

const (
	focusTabQueue     = 0
	focusTabSuggested = 1
	focusTabByProject = 2
	focusTabByDueDate = 3
)

// FocusTask is an alias for focus.FocusTask.
type FocusTask = focus.FocusTask

// ScoredFocusTask is an alias for focus.ScoredFocusTask.
type ScoredFocusTask = focus.ScoredFocusTask

// FocusViewModel displays the "What's Next" engine for cross-project task suggestions.
type FocusViewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	projectService *ProjectService

	// Tasks — all actionable tasks from all active projects
	tasks    []FocusTask
	filtered []FocusTask

	// Work queue (session-persistent, pointers to AppModel-held state)
	queue    *[]FocusTask
	queueIDs map[string]bool

	// Ranked suggestions
	ranked       []ScoredFocusTask
	dismissedIDs map[string]bool

	// Tabs
	tabs TabState

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string

	// Filter
	FilterMixin

	// Inline preview
	previewMode bool

	// Show dismissed suggestions (Suggested tab)
	showDismissed bool
}

// NewFocusViewModel creates a new focus view model.
func NewFocusViewModel(cfg *config.Config, mgr *config.Manager, queue *[]FocusTask, queueIDs map[string]bool, dismissedIDs map[string]bool) *FocusViewModel {
	m := &FocusViewModel{
		ViewBundle:     ViewBundle{config: cfg, configManager: mgr},
		projectService: NewProjectService(),
		queue:          queue,
		queueIDs:       queueIDs,
		dismissedIDs:   dismissedIDs,
		FilterMixin:    FilterMixin{filter: newFilterState()},
		tabs:           TabState{Names: focusTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.loadTasks()
	return m
}

// loadTasks aggregates all actionable tasks from all active projects.
// Filters out completed, snoozed, and blocked tasks (via HasIncompleteDependencies).
func (m *FocusViewModel) loadTasks() {
	m.tasks = nil

	var failedProjects []string
	var loadedProjects []*models.Project
	for _, project := range m.config.GetActiveProjects() {
		if err := m.projectService.Load(project); err != nil {
			failedProjects = append(failedProjects, project.Name)
			continue
		}
		loadedProjects = append(loadedProjects, project)
	}

	m.tasks = focus.CollectActionableTasks(loadedProjects)

	if len(failedProjects) > 0 {
		m.setStatus(fmt.Sprintf("Failed to load: %s", strings.Join(failedProjects, ", ")))
	}

	// Reconcile queue task pointers: replace stale pointers with freshly loaded ones.
	// When tasks are modified through other views (e.g. completing via project tree),
	// the queue holds stale *Task pointers from initial hydration.
	if len(*m.queue) > 0 {
		// Build lookup from freshly loaded tasks
		taskByID := make(map[string]FocusTask, len(m.tasks))
		for _, ft := range m.tasks {
			taskByID[ft.Task.ID] = ft
		}
		q := *m.queue
		i := 0
		for i < len(q) {
			if fresh, ok := taskByID[q[i].Task.ID]; ok {
				q[i] = fresh
				i++
			} else {
				// Task no longer actionable (completed, snoozed, etc.) — remove from queue
				delete(m.queueIDs, q[i].Task.ID)
				q = append(q[:i], q[i+1:]...)
			}
		}
		*m.queue = q
	}

	m.sortTasks()

	// Prune stale dismissed IDs (tasks that no longer exist or are no longer actionable)
	if len(m.dismissedIDs) > 0 {
		actionableIDs := make(map[string]bool, len(m.tasks))
		for _, ft := range m.tasks {
			actionableIDs[ft.Task.ID] = true
		}
		pruned := false
		for id := range m.dismissedIDs {
			if !actionableIDs[id] {
				delete(m.dismissedIDs, id)
				pruned = true
			}
		}
		if pruned {
			m.persistDismissed()
		}
	}

	// Compute ranked suggestions
	m.recomputeSuggested()
}

// recomputeSuggested re-ranks suggestions, respecting showDismissed mode.
func (m *FocusViewModel) recomputeSuggested() {
	if m.showDismissed {
		// Show only dismissed items
		all := focus.RankCrossProjectTasks(m.tasks, nil, m.config)
		m.ranked = nil
		for _, sft := range all {
			if m.dismissedIDs[sft.Task.ID] {
				m.ranked = append(m.ranked, sft)
			}
		}
	} else {
		m.ranked = focus.RankCrossProjectTasks(m.tasks, m.dismissedIDs, m.config)
	}
	m.applyFilter()
}

// applyFilter filters and sorts tasks based on the active tab, then applies sub-tab narrowing.
func (m *FocusViewModel) applyFilter() {
	now := time.Now()
	today := util.StartOfDay(now)

	switch m.tabs.Active {
	case focusTabQueue:
		// Prune completed/canceled tasks from the queue
		pruned := false
		q := *m.queue
		i := 0
		for i < len(q) {
			if q[i].Task.Status.IsComplete() {
				delete(m.queueIDs, q[i].Task.ID)
				q = append(q[:i], q[i+1:]...)
				pruned = true
			} else {
				i++
			}
		}
		if pruned {
			*m.queue = q
			m.persistQueue()
		}
		// Show the work queue
		m.filtered = make([]FocusTask, len(*m.queue))
		copy(m.filtered, *m.queue)
		return

	case focusTabSuggested:
		// Show ranked suggestions, applying common filters.
		// Non-queued items appear first, then queued items at the bottom.
		m.filtered = nil
		var queued []FocusTask
		for _, sft := range m.ranked {
			if hasActiveCommonFilters(&m.filter) {
				if !passesCommonFilters(sft.Task, &m.filter, sft.Project.ID, m.config) {
					continue
				}
			}
			if m.isInQueue(sft.Task.ID) {
				queued = append(queued, sft.FocusTask)
			} else {
				m.filtered = append(m.filtered, sft.FocusTask)
			}
		}
		m.filtered = append(m.filtered, queued...)
		return

	default:
		// By Project and By Due Date tabs work on the full task set
	}

	// Apply common filters first (type, status, priority, tags, attrs, score)
	baseTasks := m.tasks
	if hasActiveCommonFilters(&m.filter) {
		baseTasks = nil
		for _, ft := range m.tasks {
			if passesCommonFilters(ft.Task, &m.filter, ft.Project.ID, m.config) {
				baseTasks = append(baseTasks, ft)
			}
		}
	}

	switch m.tabs.Active {
	case focusTabByProject:
		m.filtered = make([]FocusTask, len(baseTasks))
		copy(m.filtered, baseTasks)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Project.Name < m.filtered[j].Project.Name
		})
	case focusTabByDueDate:
		m.filtered = nil
		for _, ft := range baseTasks {
			if ft.Task.DueDate == nil {
				continue
			}
			m.filtered = append(m.filtered, ft)
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
			switch m.tabs.Active {
			case focusTabByProject:
				if ft.Project.Name == filter {
					narrowed = append(narrowed, ft)
				}
			case focusTabByDueDate:
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
	today := util.StartOfDay(now)

	switch m.tabs.Active {
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

	case focusTabByDueDate:
		bucketOrder := []string{"Overdue", "Today", "Tomorrow", "This Week", "Later"}
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
func (m *FocusViewModel) dueBucket(task *models.Task, _, today time.Time) string {
	if task.DueDate == nil {
		return "Later"
	}
	dueDay := util.StartOfDay(*task.DueDate)
	// Compare calendar dates to avoid DST boundary skew.
	y1, m1, d1 := today.Date()
	y2, m2, d2 := dueDay.Date()
	todayUTC := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	dueUTC := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	daysUntil := int(dueUTC.Sub(todayUTC).Hours() / 24)
	switch {
	case daysUntil < 0:
		return "Overdue"
	case daysUntil == 0:
		return "Today"
	case daysUntil == 1:
		return "Tomorrow"
	case daysUntil <= 7:
		return "This Week"
	default:
		return "Later"
	}
}

// sortTasks sorts tasks by priority (ascending, 1=highest), then by UpdatedAt (most recent first).
func (m *FocusViewModel) sortTasks() {
	sort.Slice(m.tasks, func(i, j int) bool {
		if m.tasks[i].Task.Priority != m.tasks[j].Task.Priority {
			return m.tasks[i].Task.Priority < m.tasks[j].Task.Priority
		}
		return m.tasks[i].Task.UpdatedAt.After(m.tasks[j].Task.UpdatedAt)
	})
}

// getTaskAtCursor returns the task at the current cursor position.
func (m *FocusViewModel) getTaskAtCursor() *FocusTask {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

func (m *FocusViewModel) Init() tea.Cmd {
	return nil
}

func (m *FocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, func(s string) { m.setStatus(s) }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case FocusViewRefreshMsg:
		m.loadTasks()
		m.clampCursor()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *FocusViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.clearStatus()

	// Handle confirm dialog
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return taskOpsDeleteTask(m)
		}
		return m, nil
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmct.", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		RunEscCascade(m.escSteps())
		return m, nil

	case "enter":
		if ft := m.getTaskAtCursor(); ft != nil {
			return m, func() tea.Msg {
				return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project}
			}
		}
		return m, nil

	// Accept into queue (Suggested tab) or accept top suggestion (Queue tab if empty)
	case "a":
		return m.handleAccept()

	// Dismiss (Suggested tab) or remove from queue (Queue tab)
	case "x":
		return m.handleDismissOrRemove()

	// Reorder queue down
	case "J":
		if m.tabs.Active == focusTabQueue {
			q := *m.queue
			if m.cursor >= 0 && m.cursor < len(q)-1 {
				m.reorderQueue(m.cursor, m.cursor+1)
				m.cursor++
				m.applyFilter()
			}
		}
		return m, nil

	// Reorder queue up
	case "K":
		if m.tabs.Active == focusTabQueue {
			q := *m.queue
			if m.cursor > 0 && m.cursor < len(q) {
				m.reorderQueue(m.cursor, m.cursor-1)
				m.cursor--
				m.applyFilter()
			}
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
			CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
			m.applyFilter()
			m.clampCursor()
		} else {
			m.tabs.Cycle(-1)
			m.subTab = 0
			m.subTabRowFocused = false
			m.showDismissed = false
			m.recomputeSuggested()
			m.clampCursor()
		}
		return m, nil

	case "]", "l":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
			m.applyFilter()
			m.clampCursor()
		} else {
			m.tabs.Cycle(1)
			m.subTab = 0
			m.subTabRowFocused = false
			m.showDismissed = false
			m.recomputeSuggested()
			m.clampCursor()
		}
		return m, nil

	// Refresh: on Suggested tab clears dismissed and reloads; on other tabs just reloads
	case "r":
		if m.tabs.Active == focusTabSuggested {
			m.showDismissed = false
			for k := range m.dismissedIDs {
				delete(m.dismissedIDs, k)
			}
			m.persistDismissed()
		}
		m.loadTasks()
		m.clampCursor()
		m.setStatus("Refreshed")
		return m, nil

	// Toggle active flag (remove from focus)
	case "*":
		if ft := m.getTaskAtCursor(); ft != nil {
			return taskOpsRemoveFromFocus(m, ft.Task.ShortID)
		}
		return m, nil

	// Priority changes
	case "1", "2", "3", "4", "5":
		return taskOpsChangePriority(m, key)

	// Toggle show-dismissed on Suggested tab (intercept before common D=delete)
	case "D":
		if m.tabs.Active == focusTabSuggested {
			m.showDismissed = !m.showDismissed
			m.recomputeSuggested()
			m.clampCursor()
			return m, nil
		}
	}

	// Common task actions (tags, snooze, delete, edit, refresh, copy, help)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handleAccept accepts a task into the work queue.
func (m *FocusViewModel) handleAccept() (tea.Model, tea.Cmd) {
	switch m.tabs.Active {
	case focusTabQueue:
		// On Queue tab, accept the top non-queued suggestion
		if sft := m.topNonQueuedSuggestion(); sft != nil {
			m.acceptIntoQueue(sft.FocusTask)
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Added to queue: %s", sft.Task.Title))
		}
	default:
		// On all other tabs, accept the cursor item
		if ft := m.getTaskAtCursor(); ft != nil {
			m.acceptIntoQueue(*ft)
			m.applyFilter()
			m.setStatus(fmt.Sprintf("Added to queue: %s", ft.Task.Title))
		}
	}
	return m, nil
}

// handleDismissOrRemove dismisses a suggestion or removes from queue.
func (m *FocusViewModel) handleDismissOrRemove() (tea.Model, tea.Cmd) {
	switch m.tabs.Active {
	case focusTabSuggested:
		if ft := m.getTaskAtCursor(); ft != nil {
			if m.showDismissed && m.dismissedIDs[ft.Task.ID] {
				// Restore a dismissed item
				delete(m.dismissedIDs, ft.Task.ID)
				m.persistDismissed()
				m.recomputeSuggested()
				m.clampCursor()
				m.setStatus("Restored suggestion")
			} else {
				// Dismiss the item
				m.dismissedIDs[ft.Task.ID] = true
				m.persistDismissed()
				m.recomputeSuggested()
				m.clampCursor()
				m.setStatus("Dismissed suggestion")
			}
		}
	case focusTabQueue:
		if m.cursor >= 0 && m.cursor < len(*m.queue) {
			title := (*m.queue)[m.cursor].Task.Title
			m.removeFromQueue(m.cursor)
			m.applyFilter()
			m.clampCursor()
			m.setStatus(fmt.Sprintf("Removed from queue: %s", title))
		}
	default:
		m.setStatus("Use x on Queue to remove or Suggested to dismiss")
	}
	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil
	}

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		if pfx == "z" && k == "p" {
			m.previewMode = !m.previewMode
			return m, nil, true
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	return m, nil
}

// taskOpsView implementation for FocusViewModel.

func (m *FocusViewModel) getTaskRef() (*models.Task, *models.Project) {
	ft := m.getTaskAtCursor()
	if ft == nil {
		return nil, nil
	}
	return ft.Task, ft.Project
}

func (m *FocusViewModel) getPersister() Persister           { return m.projectService }
func (m *FocusViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *FocusViewModel) getDataDir() string                { return m.config.DataDir }
func (m *FocusViewModel) reloadData()                       { m.loadTasks() }
func (m *FocusViewModel) setErr(err error)                  { m.err = err }
func (m *FocusViewModel) setStatus(msg string)              { m.statusMessage = msg }

// prefixDispatchView implementation.

func (m *FocusViewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *FocusViewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return FocusViewSelectMsg{Task: task, Project: project}
}

func (m *FocusViewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleFocusFilterPrefix(key)
}

func (m *FocusViewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleFocusFilterSubPrefix(prefix, key)
}

// clampCursor ensures cursor is within valid range.
func (m *FocusViewModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

// escSteps defines the Esc dismissal cascade: filters.
func (m *FocusViewModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.showDismissed }, Clear: func() {
			m.showDismissed = false
			m.recomputeSuggested()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, or confirm dialog are active.
func (m *FocusViewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}

func (m *FocusViewModel) viewID() ViewType { return ViewFocusView }
func (m *FocusViewModel) viewName() string { return "What's Next" }

// GetHelpItems returns help items for this view.
func (m *FocusViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusView)
}

func (m *FocusViewModel) StatusBarLabel() string { return "What's Next" }
