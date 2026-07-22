package tui

import (
	"slices"
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

	tasks := filterTaskTree(roots, m.baseFocusFilter(m.project))

	if tag := m.activePinnedTagFilter(); tag != "" {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return task.HasTag(tag)
		})
	}

	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return passesCommonFilters(task, &m.filter.CommonFilterState, m.project, m.runtime.Automation())
		})
	}

	if m.activeCriteriaFilter != nil {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return passesSavedFilterCriteria(task, m.project, m.activeCriteriaFilter, m.runtime.Automation())
		})
	}

	if len(m.filter.Sections) > 0 {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return m.filter.Sections[taskSectionBucket(task)]
		})
	}

	if m.filter.Actionable {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return taskIsActionableState(task, m.project)
		})
	}

	if query := m.SearchQuery(); query != "" {
		tasks = filterTaskTree(tasks, func(task *models.Task) bool {
			return matchesSearch(task, query, m.runtime.Planning())
		})
	}
	tasks = m.applyFocusOrdering(tasks, m.project.ID)

	if m.sort.Field != SortByDefault {
		tasks = m.sortTaskTree(tasks)
	}

	if m.smartFocus == nil && m.currentPerspective().Name == "All" {
		m.buildRowsWithSectionHeaders(tasks)
	} else {
		m.flattenTaskTreeWithFolds(tasks, nil, &m.rows)
		m.insertDepth0Separators()
	}
	m.pruneInvisibleSelections()
	m.clampCursor()

	if m.OpenAtTask != nil && len(m.rows) > 0 {
		m.NavigateToSidebarTask(m.OpenAtTask)
		m.OpenAtTask = nil
	}
}

// buildRowsWithSectionHeaders groups tasks by GTD section and inserts
// collapsible section headers for the All perspective.
func (m *OutlinerModel) buildRowsWithSectionHeaders(tasks []*models.Task) {
	forests := buildSectionTaskForests(tasks)

	for _, sectionName := range BuildSectionOrder(tasks) {
		forest := forests[sectionName]
		if forest == nil || forest.count == 0 {
			continue
		}

		if len(m.rows) > 0 {
			m.rows = append(m.rows, outlinerRow{IsSeparator: true})
		}
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    forest.count,
		})

		if !m.collapsedSections[sectionName] {
			m.flattenSectionForest(forest, "", &m.rows)
		}
	}
}

// effectiveDisplaySection returns the task's display section for non-header
// contexts such as sidebar matching and task-form dimming.
func effectiveDisplaySection(t *models.Task) string {
	return taskSectionBucket(t)
}

func renderedHeaderSectionForTask(task *models.Task) string {
	section := headerSectionForTask(task)
	if section == "" || isRenderedOutlinerHeaderSection(section) {
		return section
	}
	return ""
}

func isRenderedOutlinerHeaderSection(section string) bool {
	return slices.Contains(SectionOrder, section)
}

// currentPerspective returns the active perspective preset.
// Defaults to the "All" perspective (key 1) when activePerspective is unset or negative.
func (m *OutlinerModel) currentPerspective() Perspective {
	idx := m.activePerspective
	if idx < 0 || idx > 9 {
		idx = 1
	}
	perspective, _ := PerspectiveByID(idx)
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
		cmp := CompareTasks(sorted[i], sorted[j], m.sort.Field, m.project.ID, m.runtime.Automation())
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

// clampCursor ensures the cursor stays within [0, len(rows)-1] and never
// rests on a non-navigable row (project sub-header or separator). The cursor
// is allowed to settle on section headers (which act as collapse toggles) and
// task rows. Project sub-headers are visual-only and must never be selected
// after a rebuild, filter, collapse, paging, or perspective change.
func (m *OutlinerModel) clampCursor() {
	m.ClampCursorTo(len(m.rows))
	if len(m.rows) == 0 {
		return
	}
	if m.rows[m.cursor].isNavigable() {
		return
	}
	for i := m.cursor + 1; i < len(m.rows); i++ {
		if m.rows[i].isNavigable() {
			m.cursor = i
			return
		}
	}
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].isNavigable() {
			m.cursor = i
			return
		}
	}
}

const crossProjectPinnedTagTabKey = "_cross_project_pinned_tag_tab"

// pinnedTagTabs returns ["All"] + pinned tag labels, or nil if no tags are pinned.
func (m *OutlinerModel) pinnedTagTabs() []string {
	if m.project == nil {
		return GlobalPinnedTagTabsForProjects(m.runtime.Settings(), m.loadedProjects)
	}
	return PinnedTagTabs(m.project)
}

// pinnedTagTabIndex returns the active tab index for the current outliner scope.
func (m *OutlinerModel) pinnedTagTabIndex() int {
	if m.pinnedTagTab == nil {
		return 0
	}
	return m.pinnedTagTab[m.pinnedTagTabStorageKey()]
}

// setPinnedTagTabIndex sets the active tab index for the current outliner scope.
func (m *OutlinerModel) setPinnedTagTabIndex(i int) {
	if m.pinnedTagTab == nil {
		m.pinnedTagTab = make(map[string]int)
	}
	m.pinnedTagTab[m.pinnedTagTabStorageKey()] = i
}

// activePinnedTagFilter returns the selected pinned tag, or "" for "All".
func (m *OutlinerModel) activePinnedTagFilter() string {
	tabs := m.pinnedTagTabs()
	tabIndex := m.pinnedTagTabIndex()
	if tabs == nil || tabIndex <= 0 || tabIndex >= len(tabs) {
		if tabIndex != 0 {
			m.setPinnedTagTabIndex(0)
		}
		return ""
	}
	return pinnedTagFromTab(tabs[tabIndex])
}

func (m *OutlinerModel) crossProjectPinnedTagTabBeforeLoad() (string, bool) {
	if m.project != nil || m.pinnedTagTab == nil {
		return "", false
	}
	if _, ok := m.pinnedTagTab[crossProjectPinnedTagTabKey]; !ok {
		return "", false
	}
	return m.activePinnedTagFilter(), true
}

func (m *OutlinerModel) preserveCrossOutlinerPinnedTagTabForReload() {
	if m.project != nil || m.crossReloadTabSet {
		return
	}
	tabName, ok := m.crossProjectPinnedTagTabBeforeLoad()
	if !ok {
		return
	}
	m.crossReloadTabName = tabName
	m.crossReloadTabSet = true
}

func (m *OutlinerModel) consumeCrossOutlinerReloadPinnedTagTab() (string, bool) {
	if !m.crossReloadTabSet {
		return "", false
	}
	tabName := m.crossReloadTabName
	m.crossReloadTabName = ""
	m.crossReloadTabSet = false
	return tabName, true
}

func (m *OutlinerModel) restoreCrossOutlinerPinnedTagTab(tabName string) {
	if m.project != nil {
		return
	}
	m.selectPinnedTagTab(tabName)
}

func (m *OutlinerModel) selectPinnedTagTab(tabName string) {
	tag := models.NormalizeTag(tabName)
	if tabName == "" {
		m.setPinnedTagTabIndex(0)
		return
	}
	for i, tab := range m.pinnedTagTabs() {
		if pinnedTagFromTab(tab) == tag {
			m.setPinnedTagTabIndex(i)
			return
		}
	}
	m.setPinnedTagTabIndex(0)
}

func (m *OutlinerModel) pinnedTagTabStorageKey() string {
	if m.project == nil {
		return crossProjectPinnedTagTabKey
	}
	return m.project.ID
}

// searchTagCache returns all unique tags across the outliner's project(s).
func (m *OutlinerModel) searchTagCache() []string {
	if m.project != nil {
		return CollectTagNamesForProject(m.project)
	}
	return CollectTagNamesForProjects(m.loadedProjects)
}

// searchSectionCache returns all unique sections across the outliner's project(s).
func (m *OutlinerModel) searchSectionCache() []string {
	if m.project != nil {
		return gtdSections(m.project.GetSections())
	}
	seen := make(map[string]bool)
	var result []string
	for _, p := range m.loadedProjects {
		for _, s := range gtdSections(p.GetSections()) {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}
	return result
}

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header, optional tab bar, and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 1
	if m.pinnedTagTabs() != nil {
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
