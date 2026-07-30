package tui

import "github.com/sglavoie/tt/internal/models"

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
// active mode and filters, and clamps the cursor to stay within bounds.
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
	openAt := m.OpenAtTask
	// Consume the pending target before any navigation helper can trigger a
	// nested rebuild. A target hidden by the current focus or filters otherwise
	// re-enters this block indefinitely.
	m.OpenAtTask = nil
	m.rows = m.rows[:0]

	m.validateZoomStack()

	m.buildCanonicalModeRows()

	if openAt != nil {
		m.revealTask(openAt)
	}
}

// effectiveDisplaySection returns the task's display section for non-header
// contexts such as navigation matching and task-form dimming.
func effectiveDisplaySection(t *models.Task) string {
	return taskStateGroupLabel(t)
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

// clampCursor ensures the cursor stays within [0, len(rows)-1] and never
// rests on a non-navigable row (a separator). The cursor is allowed to settle
// on section headers (which act as collapse toggles), project sub-headers
// (Enter scopes, Shift+Enter administers), and task rows. Separators are
// visual-only and must never be selected after a rebuild, filter, collapse,
// paging, or mode change.
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

// pinnedTagTabs is inert because tag pin catalogs are retired.
func (m *OutlinerModel) pinnedTagTabs() []string {
	return nil
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

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header, optional tab bar, and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 1
	if m.pinnedTagTabs() != nil {
		headerLines += 2
	}
	// The cross-project breadcrumb renders on one line whenever the view is
	// zoomed OR a project scope is active (the scope chrome), so budget a single
	// line for either/both — crossProjectBreadcrumb combines them onto one line.
	if (m.project != nil && len(m.zoomStack) > 0) ||
		(m.project == nil && (len(m.crossZoomStack) > 0 || m.isProjectScoped())) {
		headerLines++
	}
	height := m.height - headerLines
	if height < 1 {
		return 1
	}
	return height
}
