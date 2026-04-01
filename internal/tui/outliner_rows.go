package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// validateCrossZoomStack walks the cross-project zoom stack from top to bottom,
// popping entries whose task no longer exists in their owning project. It
// branches on CrossProjectZoomEntry.ProjectID to scope each lookup to the
// correct project, preventing UUID collisions across projects from causing
// incorrect resolution (per ac-d348683eb1a3e343887e804dbc6a719c).
func (m *OutlinerModel) validateCrossZoomStack() {
	if len(m.crossZoomStack) == 0 {
		return
	}

	taskExists := func(entry CrossProjectZoomEntry) bool {
		for _, proj := range m.loadedProjects {
			if proj.ID == entry.ProjectID {
				return proj.GetTaskByID(entry.TaskID) != nil
			}
		}
		return false
	}

	validDepth := -1
	for i := len(m.crossZoomStack) - 1; i >= 0; i-- {
		if taskExists(m.crossZoomStack[i]) {
			validDepth = i
			break
		}
	}

	if validDepth == len(m.crossZoomStack)-1 {
		return
	}

	if validDepth < 0 {
		m.crossZoomStack = m.crossZoomStack[:0]
		m.cursor = 0
		m.zoomNotice = "Zoom target was removed — returned to root"
		return
	}

	m.cursor = m.crossZoomStack[validDepth].CursorIndex
	m.crossZoomStack = m.crossZoomStack[:validDepth+1]
	m.zoomNotice = "Zoom target was removed — returned to parent"
}

// validateZoomStack walks the zoom stack from top to bottom, popping entries
// whose task no longer exists in the project. If any entries were removed, it
// sets a transient notice so the user knows their zoom target was deleted.
func (m *OutlinerModel) validateZoomStack() {
	if m.project == nil || len(m.zoomStack) == 0 {
		return
	}

	validDepth := -1
	for i := len(m.zoomStack) - 1; i >= 0; i-- {
		if m.project.GetTaskByID(m.zoomStack[i].TaskID) != nil {
			validDepth = i
			break
		}
	}

	if validDepth == len(m.zoomStack)-1 {
		return
	}

	if validDepth < 0 {
		m.zoomStack = m.zoomStack[:0]
		m.cursor = 0
		m.zoomNotice = "Zoom target was removed — returned to root"
		return
	}

	m.cursor = m.zoomStack[validDepth].CursorIndex
	m.zoomStack = m.zoomStack[:validDepth+1]
	m.zoomNotice = "Zoom target was removed — returned to parent"
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active perspective filter, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) rebuildRows() {
	if m.project == nil {
		// Mark as a mutation-driven rebuild so the memoization guard in
		// crossProjectRebuildRows is bypassed. The render path calls
		// crossProjectRebuildRows directly (not via rebuildRows), so it
		// still benefits from the guard on no-op events like window resize.
		m.crossForceRebuild = true
		m.crossProjectRebuildRows()
		return
	}
	m.rows = m.rows[:0]

	m.validateZoomStack()

	roots := m.project.GetRootTasks()
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if task := m.project.GetTaskByID(topID); task != nil {
			roots = task.Children
		}
	}

	tasks := filterTaskTree(roots, m.currentPerspective().Filter)

	if section := m.activeCustomSectionFilter(); section != "" {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return task.CustomSection == section
		})
	}

	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return passesCommonFilters(task, &m.filter.CommonFilterState, m.project.ID, m.config)
		})
	}

	if m.activeCriteriaFilter != nil {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return passesSavedFilterCriteria(task, m.project, m.activeCriteriaFilter, m.config)
		})
	}

	if len(m.filter.Sections) > 0 {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return m.filter.Sections[task.Section] || m.filter.Sections[task.CustomSection]
		})
	}

	if m.filter.Actionable {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			if m.project.HasIncompleteDependencies(task) {
				return false
			}
			if task.Status == models.StatusBlocked {
				return false
			}
			if task.Section == models.SectionInbox || task.Section == models.SectionNext ||
				task.Section == models.SectionWaiting || task.Section == models.SectionSomeday ||
				task.Section == models.SectionReview || task.Section == models.SectionStandby {
				return false
			}
			return true
		})
	}

	if query := m.SearchQuery(); query != "" {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return matchesSearch(task, query)
		})
	}

	if m.sort.Field != SortByDefault {
		tasks = m.sortTaskTree(tasks)
	}

	if m.currentPerspective().Name == "All" {
		m.buildRowsWithSectionHeaders(tasks)
	} else {
		m.flattenTaskTreeWithFolds(tasks, nil, &m.rows)
	}
	m.clampCursor()

	if m.OpenAtTask != nil && len(m.rows) > 0 {
		m.NavigateToSidebarTask(m.OpenAtTask)
		m.OpenAtTask = nil
	}
}

// buildRowsWithSectionHeaders groups tasks by GTD section and inserts collapsible
// section headers for the All perspective.
func (m *OutlinerModel) buildRowsWithSectionHeaders(tasks []*models.Task) {
	tasksBySection := m.groupTasksBySection(tasks)

	for _, sectionName := range SectionOrder {
		sectionTasks := tasksBySection[sectionName]
		if len(sectionTasks) == 0 {
			continue
		}

		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    m.countTasksRecursive(sectionTasks),
		})

		if !m.collapsedSections[sectionName] {
			m.flattenTaskTreeWithFolds(sectionTasks, nil, &m.rows)
		}
	}
}

// groupTasksBySection groups tasks by their GTD section, preserving hierarchy.
func (m *OutlinerModel) groupTasksBySection(tasks []*models.Task) map[string][]*models.Task {
	groups := make(map[string][]*models.Task)
	for _, task := range tasks {
		groups[task.Section] = append(groups[task.Section], task)
	}
	return groups
}

// countTasksRecursive counts all tasks in a tree, including descendants.
func (m *OutlinerModel) countTasksRecursive(tasks []*models.Task) int {
	count := 0
	for _, task := range tasks {
		count++
		count += m.countTasksRecursive(task.Children)
	}
	return count
}

// currentPerspective returns the active perspective preset.
// Defaults to the "All" perspective (key 1) when activePerspective is unset or negative.
func (m *OutlinerModel) currentPerspective() Perspective {
	idx := m.activePerspective
	if idx < 0 || idx > 9 {
		idx = 1
	}
	perspective, _ := PerspectiveByKey(idx)
	return perspective
}

// filterTaskTree returns a copy of the task tree containing only tasks that
// match the predicate or have descendants that match. Parent tasks are kept
// visible when any child matches, preserving hierarchy context.
func filterTaskTree(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		filteredChildren := filterTaskTree(task.Children, predicate)
		if predicate(task) || len(filteredChildren) > 0 {
			clone := *task
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// sortTaskTree returns a sorted copy of the task tree. Sorting is applied at
// each hierarchy level (siblings are reordered but parent-child relationships
// are preserved). The original slices are not mutated.
func (m *OutlinerModel) sortTaskTree(tasks []*models.Task) []*models.Task {
	sorted := make([]*models.Task, len(tasks))
	copy(sorted, tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := CompareTasks(sorted[i], sorted[j], m.sort.Field, m.project.ID, m.config)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
	for idx, task := range sorted {
		if len(task.Children) == 0 {
			continue
		}
		clone := *task
		clone.Children = m.sortTaskTree(task.Children)
		sorted[idx] = &clone
	}
	return sorted
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *OutlinerModel) toggleSort(field SortField) {
	m.sort.Toggle(field)
	m.rebuildRows()
}

// clampCursor ensures the cursor stays within [0, len(rows)-1].
func (m *OutlinerModel) clampCursor() {
	m.ClampCursorTo(len(m.rows))
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *OutlinerModel) customSectionTabs() []string {
	return CustomSectionTabs(m.project)
}

// customSectionTabIndex returns the active tab index for m.project.
// Must only be called when m.project != nil (i.e. customSectionTabs() != nil).
func (m *OutlinerModel) customSectionTabIndex() int {
	return m.customSectionTab[m.project.ID]
}

// setCustomSectionTabIndex sets the active tab index for m.project.
func (m *OutlinerModel) setCustomSectionTabIndex(i int) {
	m.customSectionTab[m.project.ID] = i
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *OutlinerModel) activeCustomSectionFilter() string {
	return ActiveCustomSectionFilter(m.project, m.customSectionTabIndex())
}

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header, optional tab bar, and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 1
	if m.customSectionTabs() != nil {
		headerLines += 2
	}
	if (m.project != nil && len(m.zoomStack) > 0) || (m.project == nil && len(m.crossZoomStack) > 0) {
		headerLines++
	}
	height := m.height - headerLines
	if height < 1 {
		return 1
	}
	return height
}
