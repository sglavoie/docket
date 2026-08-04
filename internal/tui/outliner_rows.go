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

// filterTaskTreeKeepingAncestors returns a copy of the task tree containing
// matching tasks plus their ancestors. It is reserved for mode membership
// projections, where a qualifying descendant may keep hierarchy context.
func filterTaskTreeKeepingAncestors(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
		filteredChildren := filterTaskTreeKeepingAncestors(task.Children, predicate)
		if predicate(task) || len(filteredChildren) > 0 {
			clone := *task
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// filterTaskTreePromotingChildren returns a copy containing only tasks that
// satisfy predicate. Matching descendants of a rejected task are promoted so
// facet filtering never hides valid work merely because an ancestor failed.
func filterTaskTreePromotingChildren(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	return excludeTaskTreePromotingChildren(tasks, func(task *models.Task) bool {
		return !predicate(task)
	})
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

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 1
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
