package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// validateCrossFocusStack keeps only the contiguous prefix that still describes
// one real ancestry branch in one owning project. Existence alone is not
// sufficient: an inline edit can reparent a nested Focus root while leaving its
// task ID alive, and keeping that frame would escape the branch above it.
func (m *OutlinerModel) validateCrossFocusStack() {
	if len(m.crossFocusStack) == 0 {
		return
	}

	projects := make(map[string]*models.Project, len(m.population.projects))
	for _, project := range m.population.projects {
		if project != nil {
			projects[project.ID] = project
		}
	}

	invalidIndex := len(m.crossFocusStack)
	var previous *models.Task
	previousProjectID := ""
	for index, entry := range m.crossFocusStack {
		project := projects[entry.ProjectID]
		task := (*models.Task)(nil)
		if project != nil {
			task = project.GetTaskByID(entry.TaskID)
		}
		if task == nil || (index > 0 &&
			(entry.ProjectID != previousProjectID || !focusTaskDescendsFrom(project, task, previous.ID))) {
			invalidIndex = index
			break
		}
		previous = task
		previousProjectID = entry.ProjectID
	}
	if invalidIndex == len(m.crossFocusStack) {
		return
	}

	restore := m.crossFocusStack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.crossFocusStack = m.crossFocusStack[:invalidIndex]
	m.setFocusRecoveryNotice(invalidIndex)
}

// validateFocusStack applies the same contiguous-branch rule to the
// single-project stack.
func (m *OutlinerModel) validateFocusStack() {
	if m.project == nil || len(m.focusStack) == 0 {
		return
	}

	invalidIndex := len(m.focusStack)
	var previous *models.Task
	for index, entry := range m.focusStack {
		task := m.project.GetTaskByID(entry.TaskID)
		if task == nil || (index > 0 && !focusTaskDescendsFrom(m.project, task, previous.ID)) {
			invalidIndex = index
			break
		}
		previous = task
	}
	if invalidIndex == len(m.focusStack) {
		return
	}

	restore := m.focusStack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.focusStack = m.focusStack[:invalidIndex]
	m.setFocusRecoveryNotice(invalidIndex)
}

// focusTaskDescendsFrom follows canonical ParentID links rather than transient
// projection pointers. It therefore stays correct after a reload or reparent.
func focusTaskDescendsFrom(project *models.Project, task *models.Task, ancestorID string) bool {
	if project == nil || task == nil || ancestorID == "" {
		return false
	}
	seen := make(map[string]bool)
	for current := task; current != nil && current.ParentID != nil; {
		parentID := *current.ParentID
		if parentID == ancestorID {
			return true
		}
		if seen[parentID] {
			return false
		}
		seen[parentID] = true
		current = project.GetTaskByID(parentID)
	}
	return false
}

func (m *OutlinerModel) setFocusRecoveryNotice(retainedDepth int) {
	destination := "root"
	if retainedDepth > 0 {
		destination = "parent"
	}
	m.focusNotice = "Focus target left this branch — returned to " + destination
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active mode and filters, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) rebuildRows() {
	m.syncJumpProjection()
	if m.project == nil {
		// Mark as a mutation-driven rebuild so the memoization guard in
		// crossProjectRebuildRows is bypassed. The render path calls
		// crossProjectRebuildRows directly (not via rebuildRows), so it
		// still benefits from the guard on no-op events like window resize.
		m.population.invalidate()
		m.crossProjectRebuildRows()
		return
	}
	openAt := m.OpenAtTask
	// Consume the pending target before any navigation helper can trigger a
	// nested rebuild. A target hidden by the current focus or filters otherwise
	// re-enters this block indefinitely.
	m.OpenAtTask = nil
	m.rows = m.rows[:0]
	// The single-project path has no memoization guard, so the index is dropped
	// on every pass through it, matching what the cross-project guard does when
	// it decides a pass is real.
	m.invalidateDependencyIndex()

	m.validateFocusStack()

	m.buildCanonicalModeRows()

	if openAt != nil {
		m.revealTask(openAt)
	}
}

// effectiveDisplaySection returns the task's display section for non-header
// contexts such as navigation matching and task-form dimming.
func effectiveDisplaySection(t *models.Task) string {
	return theme.TaskStateGroupLabel(t)
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
			clone := taskref.CloneTaskDetachingDependencies(task)
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
// on section headers (Enter zooms), project sub-headers (Enter scopes), and
// task rows. Separators are visual-only and must never be selected after a
// rebuild, filter, collapse, paging, or mode change.
func (m *OutlinerModel) clampCursor() {
	m.ClampCursorTo(len(m.rows))
	if len(m.rows) == 0 {
		return
	}
	if m.rows[m.Cursor].isNavigable() {
		return
	}
	for i := m.Cursor + 1; i < len(m.rows); i++ {
		if m.rows[i].isNavigable() {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.rows[i].isNavigable() {
			m.Cursor = i
			return
		}
	}
}

// contentHeight returns the number of task rows visible on screen after the
// one-row operational title and, when it is shown, the one-row menu bar above
// it. Focus and scope context are part of that title and never consume a second
// breadcrumb row.
//
// This is the one definition of the row budget, and every caller wants the
// reduced value: the window, the page size, and the suggestion budgets are all
// measured against the list the bar leaves behind.
func (m *OutlinerModel) contentHeight() int {
	height := m.Height() - 1 - m.menuBarRows()
	if height < 1 {
		return 1
	}
	return height
}
