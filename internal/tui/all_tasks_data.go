package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// hasActiveTasks returns true if any loaded task is in progress.
func (m *AllTasksViewModel) hasActiveTasks() bool {
	for _, item := range m.allTasks {
		if item.Task != nil && item.Task.IsInState(models.StateInProgress) {
			return true
		}
	}
	return false
}

// findBuiltInTabEntryIndex returns the tab entry index for the given built-in tab ID,
// or -1 if not found.
func (m *AllTasksViewModel) findBuiltInTabEntryIndex(builtInID int) int {
	for i, entry := range m.tabEntries {
		if entry.IsBuiltIn && entry.BuiltInID == builtInID {
			return i
		}
	}
	return -1
}

// loadAllTasks loads tasks from all active projects.
func (m *AllTasksViewModel) loadAllTasks() {
	m.allTasks = nil
	m.projectCache = make(map[string]*models.Project)

	// Check if the active tab is a smart list that includes completed tasks
	includeCompleted := m.activeTabIncludesCompleted()
	m.loadedWithCompleted = includeCompleted

	projects := m.runtime.ActiveProjects()
	for _, project := range projects {
		if err := m.projectService.Load(project); err != nil {
			continue
		}
		m.projectCache[project.ID] = project

		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if task.IsTerminalState() && !includeCompleted {
				continue
			}
			m.allTasks = append(m.allTasks, CrossProjectTask{Task: task, Project: project})
		}
	}

	m.applyFiltersAndGroup()

	if !m.initialTabSet {
		m.initialTabSet = true
		if m.hasActiveTasks() {
			if idx := m.findBuiltInTabEntryIndex(builtInTabActive); idx >= 0 {
				m.activeTabIdx = idx
				m.tabs.Active = idx
				m.applyFiltersAndGroup()
			}
		}
	}
}

// activeTabIncludesCompleted returns true if the current tab is a smart list
// with IncludeCompleted enabled.
func (m *AllTasksViewModel) activeTabIncludesCompleted() bool {
	entry := m.activeTabEntry()
	if entry == nil || entry.IsBuiltIn || entry.IsOverflow {
		return false
	}
	sf := m.runtime.Planning().GetSavedFilter(entry.SavedFilterID)
	return sf != nil && sf.IncludeCompleted
}

// applyFiltersAndGroup applies filters, tab filter, grouping, and sorting.
func (m *AllTasksViewModel) applyFiltersAndGroup() {
	// Step 1: Apply common filters
	filtered := m.allTasks
	if hasActiveCommonFilters(&m.filter) {
		filtered = nil
		for _, item := range m.allTasks {
			if passesCommonFilters(item.Task, &m.filter, item.Project, m.runtime.Automation()) {
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
	return m, m.SectionGoToNext(m.groupStarts)
}

// goToPrevGroup moves cursor to previous group header (with wrap).
func (m *AllTasksViewModel) goToPrevGroup() (tea.Model, tea.Cmd) {
	return m, m.SectionGoToPrev(m.groupStarts)
}

// projectCount returns the number of unique projects loaded.
func (m *AllTasksViewModel) projectCount() int {
	return len(m.projectCache)
}

// taskOpsView implementation for AllTasksViewModel.
func (m *AllTasksViewModel) getTaskRef() (*models.Task, *models.Project) {
	item := m.getTaskAtCursor()
	if item == nil {
		return nil, nil
	}
	return item.Task, item.Project
}

func (m *AllTasksViewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *AllTasksViewModel) getRuntime() *config.RuntimeState  { return m.runtime }

// findNavigableIndex returns the navigable cursor position (header + task rows)
// for the task with the given ID, or -1 if not found.
func (m *AllTasksViewModel) findNavigableIndex(taskID string) int {
	pos := 0
	for _, group := range m.groups {
		pos++ // header row
		if !m.collapsedGroups[group.Name] {
			for _, item := range group.Tasks {
				if item.Task != nil && item.Task.ID == taskID {
					return pos
				}
				pos++
			}
		}
	}
	return -1
}
func (m *AllTasksViewModel) setStatus(msg string) { m.statusMessage = msg }

// clampCursor ensures cursor is within valid range.
func (m *AllTasksViewModel) clampCursor() {
	m.ClampCursorTo(m.totalNavigableItems())
}

func (m *AllTasksViewModel) viewID() ViewType { return ViewAllTasks }
func (m *AllTasksViewModel) viewName() string { return "All Tasks" }

// GetHelpItems returns help items for this view.
func (m *AllTasksViewModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetAppLevelForView(m.viewID()),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *AllTasksViewModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// HasActiveFilters returns true if any filter is currently active.
func (m *AllTasksViewModel) HasActiveFilters() bool {
	return hasActiveCommonFilters(&m.filter) || len(m.projectFilter) > 0
}

// ActiveFilterCount returns the number of active filters. Overrides FilterMixin
// to include the project filter in the count.
func (m *AllTasksViewModel) ActiveFilterCount() int {
	return m.FilterMixin.ActiveFilterCount() + len(m.projectFilter)
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
