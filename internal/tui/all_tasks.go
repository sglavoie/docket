package tui

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// AllTasksLayoutMode controls list vs kanban rendering for the all tasks view.
type AllTasksLayoutMode int

const (
	ATLayoutList   AllTasksLayoutMode = iota
	ATLayoutKanban                    // horizontal columns per group
)

// AllTasksGroupBy represents the grouping strategy for the all tasks view.
type AllTasksGroupBy int

const (
	ATGroupByProject  AllTasksGroupBy = iota
	ATGroupByStatus                   // by workflow status
	ATGroupByPriority                 // by priority level
	ATGroupBySection                  // by GTD section
	ATGroupByDueDate                  // by due date bucket
	ATGroupByFlat                     // no grouping
)

func (g AllTasksGroupBy) String() string {
	switch g {
	case ATGroupByProject:
		return "Project"
	case ATGroupByStatus:
		return "Status"
	case ATGroupByPriority:
		return "Priority"
	case ATGroupBySection:
		return "Section"
	case ATGroupByDueDate:
		return "Due Date"
	case ATGroupByFlat:
		return "Flat"
	default:
		return "Project"
	}
}

// AllTasksGroup represents a named group of tasks.
type AllTasksGroup struct {
	Name  string
	Tasks []CrossProjectTask
}

var defaultBuiltInTabNames = []string{"All", "Active", "Due Soon", "High Pri", "Blocked", "By Score"}

const (
	builtInTabAll     = 0
	builtInTabActive  = 1
	builtInTabDueSoon = 2
	builtInTabHighPri = 3
	builtInTabBlocked = 4
	builtInTabByScore = 5
)

// TabEntry represents a single tab in the AllTasks view, which can be either
// a built-in filter tab or a user-defined smart list.
type TabEntry struct {
	Name        string
	IsBuiltIn   bool
	BuiltInID   int
	SmartListID string
}

// AllTasksViewModel displays all tasks across all projects.
type AllTasksViewModel struct {
	BaseView
	InteractiveViewState
	ViewBundle

	projectService *ProjectService
	projectCache   map[string]*models.Project

	// All loaded tasks (unfiltered)
	allTasks []CrossProjectTask

	// Grouped/filtered view
	groups     []AllTasksGroup
	totalItems int

	// Grouping
	groupBy         AllTasksGroupBy
	collapsedGroups map[string]bool

	// Tabs
	tabs       TabState
	tabEntries []TabEntry

	// Filter
	FilterMixin
	projectFilter map[string]bool // included project IDs (empty = all)

	// Sort
	sort SortState

	// Multi-select
	MultiSelectState

	// Navigation
	scrollOffset int
	groupStarts  []int // cursor positions of group headers

	// Layout mode (list vs kanban)
	layoutMode AllTasksLayoutMode

	// Kanban state
	kanbanColumns   map[string]*ColumnState // per-group cursor+scroll
	kanbanActiveCol int                     // index into m.groups
	kanbanColScroll int                     // first visible column index
	kanbanColWidth  int                     // calculated column width
	kanbanHeight    int                     // available height for columns
}

// NewAllTasksViewModel creates a new all tasks view model.
func NewAllTasksViewModel(cfg *config.Config, mgr *config.Manager) *AllTasksViewModel {
	m := &AllTasksViewModel{
		ViewBundle:       ViewBundle{config: cfg, configManager: mgr},
		projectService:   NewProjectService(),
		projectCache:     make(map[string]*models.Project),
		collapsedGroups:  make(map[string]bool),
		projectFilter:    make(map[string]bool),
		MultiSelectState: MultiSelectState{selectedTasks: make(map[string]bool)},
		sort:             NewSortState(),
		FilterMixin:      FilterMixin{filter: newFilterState()},
		kanbanColumns:    make(map[string]*ColumnState),
	}
	m.buildTabEntries()
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.loadAllTasks()
	return m
}

// buildTabEntries constructs the ordered list of tab entries from config.
// If AllTasksTabOrder is set, it determines ordering; otherwise built-in tabs
// come first followed by smart lists in definition order.
func (m *AllTasksViewModel) buildTabEntries() {
	builtInByName := make(map[string]int)
	for i, name := range defaultBuiltInTabNames {
		builtInByName[name] = i
	}

	smartByID := make(map[string]bool)
	for _, sl := range m.config.SmartLists {
		smartByID[sl.ID] = true
	}

	var entries []TabEntry
	used := make(map[string]bool)

	if len(m.config.AllTasksTabOrder) > 0 {
		for _, key := range m.config.AllTasksTabOrder {
			if id, ok := builtInByName[key]; ok && !used[key] {
				entries = append(entries, TabEntry{Name: key, IsBuiltIn: true, BuiltInID: id})
				used[key] = true
			} else if smartByID[key] && !used[key] {
				sl := m.config.GetSmartList(key)
				if sl != nil {
					entries = append(entries, TabEntry{Name: sl.Name, IsBuiltIn: false, SmartListID: key})
					used[key] = true
				}
			}
		}
	}

	// Append any built-in tabs not yet included (preserve default order)
	for i, name := range defaultBuiltInTabNames {
		if !used[name] {
			entries = append(entries, TabEntry{Name: name, IsBuiltIn: true, BuiltInID: i})
		}
	}

	// Append any smart lists not yet included
	for _, sl := range m.config.SmartLists {
		if !used[sl.ID] {
			entries = append(entries, TabEntry{Name: sl.Name, IsBuiltIn: false, SmartListID: sl.ID})
		}
	}

	m.tabEntries = entries

	names := make([]string, len(entries))
	for i, e := range entries {
		if !e.IsBuiltIn {
			names[i] = "● " + e.Name
		} else {
			names[i] = e.Name
		}
	}
	m.tabs.Names = names

	// Clamp active tab in case entries shrank
	if m.tabs.Active >= len(entries) {
		m.tabs.Active = 0
	}
}

// loadAllTasks loads tasks from all active projects.
func (m *AllTasksViewModel) loadAllTasks() {
	m.allTasks = nil
	m.projectCache = make(map[string]*models.Project)

	// Check if the active tab is a smart list that includes completed tasks
	includeCompleted := m.activeTabIncludesCompleted()

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		m.projectCache[project.ID] = project

		for _, task := range project.Tasks {
			if task.Status.IsComplete() && !includeCompleted {
				continue
			}
			m.allTasks = append(m.allTasks, CrossProjectTask{Task: task, Project: project})
		}
	}

	m.applyFiltersAndGroup()
}

// activeTabIncludesCompleted returns true if the current tab is a smart list
// with IncludeCompleted enabled.
func (m *AllTasksViewModel) activeTabIncludesCompleted() bool {
	if m.tabs.Active < 0 || m.tabs.Active >= len(m.tabEntries) {
		return false
	}
	entry := m.tabEntries[m.tabs.Active]
	if entry.IsBuiltIn {
		return false
	}
	sl := m.config.GetSmartList(entry.SmartListID)
	return sl != nil && sl.IncludeCompleted
}

// applyFiltersAndGroup applies filters, tab filter, grouping, and sorting.
func (m *AllTasksViewModel) applyFiltersAndGroup() {
	// Step 1: Apply common filters
	filtered := m.allTasks
	if hasActiveCommonFilters(&m.filter) {
		filtered = nil
		for _, item := range m.allTasks {
			if passesCommonFilters(item.Task, &m.filter, item.Project.ID, m.config) {
				filtered = append(filtered, item)
			}
		}
	}

	// Step 2: Apply project filter
	if len(m.projectFilter) > 0 {
		var projectFiltered []CrossProjectTask
		for _, item := range filtered {
			if m.projectFilter[item.Project.ID] {
				projectFiltered = append(projectFiltered, item)
			}
		}
		filtered = projectFiltered
	}

	// Step 3: Apply tab-specific filter
	filtered = m.applyTabFilter(filtered)

	// Step 4: Sort
	m.applySorting(filtered)

	// Step 5: Group
	m.groups = m.groupTasks(filtered)

	// Step 6: Calculate navigation (headers + visible tasks)
	m.calculateNavigation()
}

// applyTabFilter filters tasks based on the active tab entry.
func (m *AllTasksViewModel) applyTabFilter(tasks []CrossProjectTask) []CrossProjectTask {
	if m.tabs.Active < 0 || m.tabs.Active >= len(m.tabEntries) {
		return tasks
	}
	entry := m.tabEntries[m.tabs.Active]

	if entry.IsBuiltIn {
		return m.applyBuiltInTabFilter(tasks, entry.BuiltInID)
	}

	// Smart list tab
	sl := m.config.GetSmartList(entry.SmartListID)
	if sl == nil {
		return tasks
	}
	return m.applySmartListFilter(tasks, sl)
}

// applyBuiltInTabFilter applies the filter logic for built-in tabs.
func (m *AllTasksViewModel) applyBuiltInTabFilter(tasks []CrossProjectTask, builtInID int) []CrossProjectTask {
	now := time.Now()
	today := util.StartOfDay(now)

	switch builtInID {
	case builtInTabActive:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Status == models.StatusInProgress {
				result = append(result, item)
			}
		}
		return result

	case builtInTabDueSoon:
		sevenDaysLater := today.AddDate(0, 0, 7)
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.DueDate == nil {
				continue
			}
			dueDay := util.StartOfDay(*item.Task.DueDate)
			if dueDay.Before(sevenDaysLater) {
				result = append(result, item)
			}
		}
		return result

	case builtInTabHighPri:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Priority <= 2 {
				result = append(result, item)
			}
		}
		return result

	case builtInTabBlocked:
		var result []CrossProjectTask
		for _, item := range tasks {
			if item.Task.Status == models.StatusBlocked {
				result = append(result, item)
			}
		}
		return result

	case builtInTabByScore:
		var result []CrossProjectTask
		for _, item := range tasks {
			profile := m.config.GetProjectScoringProfile(item.Project.ID)
			if profile != nil && HasManualScores(item.Task, profile) {
				result = append(result, item)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			pi := m.config.GetProjectScoringProfile(result[i].Project.ID)
			pj := m.config.GetProjectScoringProfile(result[j].Project.ID)
			si := scoring.ComputeScore(result[i].Task, pi)
			sj := scoring.ComputeScore(result[j].Task, pj)
			return si > sj
		})
		return result
	}

	return tasks
}

// applySmartListFilter filters tasks using a smart list's criteria.
func (m *AllTasksViewModel) applySmartListFilter(tasks []CrossProjectTask, sl *config.SmartList) []CrossProjectTask {
	var result []CrossProjectTask
	for _, item := range tasks {
		// Smart lists skip completed tasks unless explicitly included
		if item.Task.Status.IsComplete() && !sl.IncludeCompleted {
			continue
		}
		if passesSmartListCriteria(item.Task, item.Project, sl, m.config) {
			result = append(result, item)
		}
	}
	return result
}

// calculateNavigation counts navigable items (headers + visible tasks)
// and builds groupStarts for n/N header jumping.
func (m *AllTasksViewModel) calculateNavigation() {
	m.totalItems = 0
	m.groupStarts = nil
	for _, group := range m.groups {
		m.groupStarts = append(m.groupStarts, m.totalItems)
		m.totalItems++ // header
		if !m.collapsedGroups[group.Name] {
			m.totalItems += len(group.Tasks)
		}
	}
}

// getTaskAtCursor returns the task at the current cursor position.
// Returns nil if cursor is on a group header.
func (m *AllTasksViewModel) getTaskAtCursor() *CrossProjectTask {
	if m.layoutMode == ATLayoutKanban {
		return m.getKanbanTaskAtCursor()
	}
	pos := 0
	for i := range m.groups {
		if m.cursor == pos {
			return nil // On header
		}
		pos++ // past header

		if !m.collapsedGroups[m.groups[i].Name] {
			for j := range m.groups[i].Tasks {
				if m.cursor == pos {
					return &m.groups[i].Tasks[j]
				}
				pos++
			}
		}
	}
	return nil
}

// totalNavigableItems returns the count of navigable items (headers + visible tasks).
func (m *AllTasksViewModel) totalNavigableItems() int {
	return m.totalItems
}

// totalVisibleTaskCount returns the count of visible tasks (excluding headers).
func (m *AllTasksViewModel) totalVisibleTaskCount() int {
	count := 0
	for _, group := range m.groups {
		if !m.collapsedGroups[group.Name] {
			count += len(group.Tasks)
		}
	}
	return count
}

// goToNextGroup moves cursor to next group header (with wrap).
func (m *AllTasksViewModel) goToNextGroup() (tea.Model, tea.Cmd) {
	old := m.cursor
	if pos := navigateToNextSection(m.cursor, m.groupStarts); pos >= 0 {
		m.cursor = pos
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

// goToPrevGroup moves cursor to previous group header (with wrap).
func (m *AllTasksViewModel) goToPrevGroup() (tea.Model, tea.Cmd) {
	old := m.cursor
	if pos := navigateToPrevSection(m.cursor, m.groupStarts); pos >= 0 {
		m.cursor = pos
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

// projectCount returns the number of unique projects loaded.
func (m *AllTasksViewModel) projectCount() int {
	return len(m.projectCache)
}

func (m *AllTasksViewModel) Init() tea.Cmd {
	return nil
}

func (m *AllTasksViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadAllTasks, m.clampCursor, func(s string) { m.statusMessage = s }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case AllTasksRefreshMsg:
		m.loadAllTasks()
		m.clampCursor()
		return m, nil

	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *AllTasksViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(key); handled {
			if confirmed {
				if action == "delete-smart-list" {
					return m.handleDeleteSmartList()
				}
				if m.multiSelectMode && m.SelectedCount() > 0 {
					return m.handleBulkDelete()
				}
				return taskOpsDeleteTask(m)
			}
			return m, nil
		}
	}

	// Multi-select key handling
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case " ":
			m.toggleTaskSelection()
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAll()
			return m, nil
		case "D":
			if m.SelectedCount() > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", m.SelectedCount()))
			}
			return m, nil
		}
		// Allow prefix keys in multi-select for bulk operations
		switch key {
		case ",", "!", "d", "z":
			m.pendingPrefix = key
			return m, nil
		case "m":
			m.pendingPrefix = "m"
			return m, nil
		}
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, ",!dgzF", "mc", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Kanban-specific navigation (intercept before list navigation and tab cycling)
	if m.layoutMode == ATLayoutKanban {
		if model, cmd, handled := m.handleKanbanKeyMsg(key); handled {
			return model, cmd
		}
	}

	// Navigation (list mode only — kanban intercepts j/k/G/etc. above)
	if m.layoutMode == ATLayoutList {
		if handled, cmd := handleListNavigation(key, &m.cursor, m.totalNavigableItems(), m.Height(), &m.cursorFlash); handled {
			return m, cmd
		}
	}

	switch key {
	case "ctrl+k":
		m.toggleLayout()
		return m, nil

	case "esc", "q":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseAllTasksMsg{}
		}

	case "enter":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Group header navigation (list mode only)
	case "n":
		if m.layoutMode == ATLayoutList {
			return m.goToNextGroup()
		}
	case "N":
		if m.layoutMode == ATLayoutList {
			return m.goToPrevGroup()
		}

	// Group cycling
	case "ctrl+g":
		m.cycleGroupBy()
		if m.layoutMode == ATLayoutKanban {
			// Flat grouping is degenerate in kanban — auto-switch
			if m.groupBy == ATGroupByFlat {
				m.groupBy = ATGroupByProject
				m.applyFiltersAndGroup()
				m.clampCursor()
				m.statusMessage = "Switched to Project grouping for kanban"
			}
			m.reinitKanbanColumns()
			m.ensureActiveKanbanColumnVisible()
		}
		return m, nil

	// Group collapse/expand (list mode only)
	case "tab":
		if m.layoutMode == ATLayoutList {
			m.toggleGroupCollapse()
		}
		return m, nil

	// Multi-select
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Toggle active flag
	case "*":
		return taskOpsToggleActive(m)

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sort, func() {
			m.applyFiltersAndGroup()
			m.clampCursor()
			if m.layoutMode == ATLayoutKanban {
				m.reinitKanbanColumns()
			}
			m.statusMessage = "Sort reset"
		}, func(field SortField) {
			m.applyFiltersAndGroup()
			m.clampCursor()
			if m.layoutMode == ATLayoutKanban {
				m.reinitKanbanColumns()
			}
			m.statusMessage = fmt.Sprintf("Sort: %s %s", field, m.sortDirection())
		})
		return m, nil

	// Score editor
	case "S":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Smart list: create
	case "+":
		return m, func() tea.Msg {
			return ShowSmartListBuilderMsg{}
		}

	// Smart list: edit current tab
	case "E":
		if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
			entry := m.tabEntries[m.tabs.Active]
			if !entry.IsBuiltIn {
				sl := m.config.GetSmartList(entry.SmartListID)
				if sl != nil {
					return m, func() tea.Msg {
						return ShowSmartListEditorMsg{SmartList: sl}
					}
				}
			}
		}
		return m, nil

	// Tab reordering (list mode only — kanban uses {/} for task movement)
	case "{":
		if m.layoutMode == ATLayoutList {
			m.moveTabLeft()
			return m, nil
		}
	case "}":
		if m.layoutMode == ATLayoutList {
			m.moveTabRight()
			return m, nil
		}

	// Smart list: delete current tab
	case "X":
		if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
			entry := m.tabEntries[m.tabs.Active]
			if !entry.IsBuiltIn {
				m.confirm.Show("delete-smart-list", fmt.Sprintf("Delete smart list %q?", entry.Name))
			}
		}
		return m, nil
	}

	// Tab navigation — in kanban mode, only [/] cycle tabs (h/l are column nav)
	if handleTabCycleKey(key, &m.tabs, m.layoutMode == ATLayoutList, func() {
		m.applyFiltersAndGroup()
		m.clampCursor()
		if m.layoutMode == ATLayoutKanban {
			m.reinitKanbanColumns()
			m.ensureActiveKanbanColumnVisible()
		}
	}) {
		return m, nil
	}

	// Common task actions (tags, snooze, delete, edit, refresh, copy, help)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *AllTasksViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle bulk operations in multi-select mode
	if m.multiSelectMode && m.SelectedCount() > 0 {
		switch prefix {
		case ",":
			if status, ok := StatusForPrefixKey(key); ok {
				return m.handleBulkStatusChange(status)
			}
		case "!":
			if priority, ok := PriorityForPrefixKey(key); ok {
				return m.handleBulkPriorityChange(priority)
			}
		case "d":
			if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
				if isClear {
					return m.handleBulkClearDueDate()
				}
				return m.handleBulkDueDateChange(days)
			}
		case "m":
			section, ok := sectionForMoveKey(key)
			if ok {
				return m.handleBulkSectionMove(section)
			}
		}
		return m, nil
	}

	// View-specific prefixes, then common dispatch
	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		switch pfx {
		case "z":
			handleZPrefix(k, func() {
				ExpandAllSections(m.collapsedGroups)
				m.calculateNavigation()
				m.clampCursor()
			}, func() {
				groupNames := make([]string, len(m.groups))
				for i, g := range m.groups {
					groupNames[i] = g.Name
				}
				CollapseAllSections(m.collapsedGroups, groupNames)
				m.calculateNavigation()
				if len(m.groupStarts) > 0 {
					best := m.groupStarts[0]
					for _, start := range m.groupStarts {
						if start <= m.cursor {
							best = start
						}
					}
					m.cursor = best
				}
				m.clampCursor()
			})
			return m, nil, true

		case "!":
			if _, ok := PriorityForPrefixKey(k); ok {
				model, cmd := taskOpsChangePriority(m, k)
				return model, cmd, true
			}
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	return m, nil
}

// taskOpsView implementation for AllTasksViewModel.

func (m *AllTasksViewModel) getTaskRef() (*models.Task, *models.Project) {
	item := m.getTaskAtCursor()
	if item == nil {
		return nil, nil
	}
	return item.Task, item.Project
}

func (m *AllTasksViewModel) getPersister() Persister           { return m.projectService }
func (m *AllTasksViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *AllTasksViewModel) getDataDir() string                { return m.config.DataDir }
func (m *AllTasksViewModel) reloadData()                       { m.loadAllTasks() }
func (m *AllTasksViewModel) setErr(err error)                  { m.err = err }
func (m *AllTasksViewModel) setStatus(msg string)              { m.statusMessage = msg }

// prefixDispatchView implementation.

func (m *AllTasksViewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *AllTasksViewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return AllTasksSelectMsg{Task: task, Project: project}
}

func (m *AllTasksViewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix(key)
}

func (m *AllTasksViewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterSubPrefix(prefix, key)
}

// clampCursor ensures cursor is within valid range.
func (m *AllTasksViewModel) clampCursor() {
	m.ClampCursorTo(m.totalNavigableItems())
}

// escSteps defines the Esc dismissal cascade: multi-select → filters.
func (m *AllTasksViewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.statusMessage = "Filters cleared"
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, multi-select, or confirm dialog are active.
func (m *AllTasksViewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}

func (m *AllTasksViewModel) viewID() ViewType { return ViewAllTasks }
func (m *AllTasksViewModel) viewName() string { return "All Tasks" }

// GetHelpItems returns help items for this view.
func (m *AllTasksViewModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewAllTasks)
}

// HasActiveFilters returns true if any filter is currently active.
func (m *AllTasksViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter) || len(m.projectFilter) > 0
}

// toggleProjectFilter toggles the project of the task under cursor in/out of the filter set.
func (m *AllTasksViewModel) toggleProjectFilter() {
	item := m.getTaskAtCursor()
	if item == nil {
		return
	}
	projectID := item.Project.ID
	if m.projectFilter[projectID] {
		delete(m.projectFilter, projectID)
	} else {
		m.projectFilter[projectID] = true
	}
	m.applyFiltersAndGroup()
	m.clampCursor()
}

// ClearAllFilters resets all filters to their default state.
func (m *AllTasksViewModel) ClearAllFilters() {
	clearCommonFilters(&m.filter)
	m.projectFilter = make(map[string]bool)
}

// ShowAllTasksMsg is sent to open the all tasks view.
type ShowAllTasksMsg struct{}

// CloseAllTasksMsg is sent when the all tasks view should close.
type CloseAllTasksMsg struct{}

// AllTasksSelectMsg is sent when a task is selected for navigation.
type AllTasksSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// AllTasksRefreshMsg is sent to refresh the all tasks view.
type AllTasksRefreshMsg struct{}

// toggleLayout switches between list and kanban mode.
func (m *AllTasksViewModel) toggleLayout() {
	if m.layoutMode == ATLayoutList {
		// Flat grouping produces one column which is degenerate — auto-switch.
		if m.groupBy == ATGroupByFlat {
			m.groupBy = ATGroupByProject
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.statusMessage = "Switched to Project grouping for kanban"
		}
		m.layoutMode = ATLayoutKanban
		m.initKanbanColumns()
	} else {
		m.layoutMode = ATLayoutList
		m.syncKanbanToList()
	}
}

// initKanbanColumns sets up per-group column state and positions the active
// column to whichever group contains the current list cursor.
func (m *AllTasksViewModel) initKanbanColumns() {
	m.kanbanColumns = make(map[string]*ColumnState)
	for _, g := range m.groups {
		m.kanbanColumns[g.Name] = &ColumnState{}
	}

	// Find which group the list cursor sits in and set kanbanActiveCol.
	pos := 0
	for gi, group := range m.groups {
		headerPos := pos
		pos++ // past header
		end := pos
		if !m.collapsedGroups[group.Name] {
			end = pos + len(group.Tasks)
		}
		if m.cursor >= headerPos && m.cursor < end {
			m.kanbanActiveCol = gi
			// If cursor was on a task (not header), set column cursor too.
			if m.cursor > headerPos && !m.collapsedGroups[group.Name] {
				m.kanbanColumns[group.Name].Cursor = m.cursor - headerPos - 1
			}
			break
		}
		pos = end
	}
	m.kanbanColScroll = 0
	m.ensureActiveKanbanColumnVisible()
}

// syncKanbanToList positions the list cursor on the task selected in the active
// kanban column when switching back to list mode.
func (m *AllTasksViewModel) syncKanbanToList() {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return
	}
	group := m.groups[m.kanbanActiveCol]
	cs := m.kanbanColumns[group.Name]

	pos := 0
	for gi, g := range m.groups {
		if gi == m.kanbanActiveCol {
			if cs != nil && cs.Cursor >= 0 && cs.Cursor < len(g.Tasks) {
				pos += 1 + cs.Cursor // header + offset
			}
			break
		}
		pos++ // header
		if !m.collapsedGroups[g.Name] {
			pos += len(g.Tasks)
		}
	}
	m.cursor = pos
	m.clampCursor()
}

// getKanbanTaskAtCursor returns the task under the kanban column cursor.
func (m *AllTasksViewModel) getKanbanTaskAtCursor() *CrossProjectTask {
	if m.kanbanActiveCol < 0 || m.kanbanActiveCol >= len(m.groups) {
		return nil
	}
	group := m.groups[m.kanbanActiveCol]
	cs := m.kanbanColumns[group.Name]
	if cs == nil || cs.Cursor < 0 || cs.Cursor >= len(group.Tasks) {
		return nil
	}
	return &group.Tasks[cs.Cursor]
}

// ensureTabOrderInitialized populates AllTasksTabOrder from the current
// tabEntries if it hasn't been set yet, so that reordering has something
// to permute.
func (m *AllTasksViewModel) ensureTabOrderInitialized() {
	if len(m.config.AllTasksTabOrder) > 0 {
		return
	}
	order := make([]string, len(m.tabEntries))
	for i, e := range m.tabEntries {
		if e.IsBuiltIn {
			order[i] = e.Name
		} else {
			order[i] = e.SmartListID
		}
	}
	m.config.AllTasksTabOrder = order
}

// moveTabLeft swaps the active tab with the one to its left.
func (m *AllTasksViewModel) moveTabLeft() {
	if m.tabs.Active <= 0 {
		return
	}
	m.ensureTabOrderInitialized()
	i := m.tabs.Active
	m.config.AllTasksTabOrder[i-1], m.config.AllTasksTabOrder[i] = m.config.AllTasksTabOrder[i], m.config.AllTasksTabOrder[i-1]
	m.tabs.Active--
	m.buildTabEntries()
	_ = m.configManager.Save(m.config)
	m.statusMessage = "Tab moved left"
}

// moveTabRight swaps the active tab with the one to its right.
func (m *AllTasksViewModel) moveTabRight() {
	if m.tabs.Active >= len(m.tabEntries)-1 {
		return
	}
	m.ensureTabOrderInitialized()
	i := m.tabs.Active
	m.config.AllTasksTabOrder[i], m.config.AllTasksTabOrder[i+1] = m.config.AllTasksTabOrder[i+1], m.config.AllTasksTabOrder[i]
	m.tabs.Active++
	m.buildTabEntries()
	_ = m.configManager.Save(m.config)
	m.statusMessage = "Tab moved right"
}

// handleDeleteSmartList deletes the smart list on the currently active tab.
func (m *AllTasksViewModel) handleDeleteSmartList() (tea.Model, tea.Cmd) {
	if m.tabs.Active < 0 || m.tabs.Active >= len(m.tabEntries) {
		return m, nil
	}
	entry := m.tabEntries[m.tabs.Active]
	if entry.IsBuiltIn || entry.SmartListID == "" {
		return m, nil
	}
	id := entry.SmartListID
	return m, func() tea.Msg {
		return SmartListDeletedMsg{ID: id}
	}
}
