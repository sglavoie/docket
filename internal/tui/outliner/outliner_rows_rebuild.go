package outliner

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// validateCrossFocusStack keeps only the contiguous prefix that still describes
// one real ancestry branch in one owning project. Existence alone is not
// sufficient: an inline edit can reparent a nested Focus root while leaving its
// task ID alive, and keeping that frame would escape the branch above it.
func (m *OutlinerModel) validateCrossFocusStack() {
	if len(m.Focus.CrossStack) == 0 {
		return
	}

	projects := make(map[string]*models.Project, len(m.Population.Projects))
	for _, project := range m.Population.Projects {
		if project != nil {
			projects[project.ID] = project
		}
	}

	invalidIndex := len(m.Focus.CrossStack)
	var previous *models.Task
	previousProjectID := ""
	for index, entry := range m.Focus.CrossStack {
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
	if invalidIndex == len(m.Focus.CrossStack) {
		return
	}

	restore := m.Focus.CrossStack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.Focus.truncateCross(invalidIndex)
	m.setFocusRecoveryNotice(invalidIndex)
}

// validateFocusStack applies the same contiguous-branch rule to the
// single-project stack.
func (m *OutlinerModel) validateFocusStack() {
	if m.Project == nil || len(m.Focus.Stack) == 0 {
		return
	}

	invalidIndex := len(m.Focus.Stack)
	var previous *models.Task
	for index, entry := range m.Focus.Stack {
		task := m.Project.GetTaskByID(entry.TaskID)
		if task == nil || (index > 0 && !focusTaskDescendsFrom(m.Project, task, previous.ID)) {
			invalidIndex = index
			break
		}
		previous = task
	}
	if invalidIndex == len(m.Focus.Stack) {
		return
	}

	restore := m.Focus.Stack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.Focus.truncate(invalidIndex)
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
	m.Focus.setNotice("Focus target left this branch — returned to " + destination)
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active mode and filters, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) RebuildRows() {
	m.Search.cancelBackground()
	m.Population.Invalidate()
	m.rebuildRows()
}

// rebuildRows is also used to publish a search request/result without declaring
// a new workspace revision and invalidating that very request.
func (m *OutlinerModel) rebuildRows() {
	m.feedbackBudget = feedbackBudgetAnchor{}
	// The multi-select summary counts visible against hidden members, so it is a
	// projection of the rows and not of the selection alone: a fold, a filter, or
	// a mode change moves members across that line without touching the selection.
	// Nothing is refreshed here, because the summary is a context-row segment
	// derived from the live rows at render time rather than a status message some
	// seam has to keep current — which is what makes a stale count unreachable
	// instead of merely refreshed at every seam that remembered to.
	defer m.restoreTimeDrivenCursor()
	m.SyncJumpProjection()
	if m.Project == nil {
		// Mark as a mutation-driven rebuild so the memoization guard in
		// crossProjectRebuildRows is bypassed. The render path calls
		// crossProjectRebuildRows directly (not via rebuildRows), so it
		// still benefits from the guard on no-op events like window resize.
		m.Population.ForceRebuild = true
		m.CrossProjectRebuildRows()
		return
	}
	defer func() {
		m.Population.record(m.singleProjectRowsFingerprint(), len(m.Focus.Stack), len(m.Project.Tasks))
	}()
	openAt := m.OpenAtTask
	// Consume the pending target before any navigation helper can trigger a
	// nested rebuild. A target hidden by the current focus or filters otherwise
	// re-enters this block indefinitely.
	m.OpenAtTask = nil
	m.Rows = m.Rows[:0]
	m.hiddenLayout.reset()
	// Explicit rebuilds invalidate the index; repaint-only frames reuse both.
	m.invalidateDependencyIndex()

	m.validateFocusStack()

	m.buildCanonicalModeRows()
	m.reseatNoteCursorAfterRebuild()

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

// filterTaskTreePromotingChildren keeps only the tasks that satisfy predicate.
// Matching descendants of a rejected task are promoted so facet filtering never
// hides valid work merely because an ancestor failed. It is a pass over a tree
// the projection owns, so it shares what it leaves alone; see pruneOwnedTree.
func filterTaskTreePromotingChildren(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	pruned, _ := pruneOwnedTree(tasks, func(task *models.Task) bool {
		return !predicate(task)
	})
	return pruned
}

// clampCursor ensures the cursor stays within [0, len(rows)-1] and never
// rests on a non-navigable row (a separator). The cursor is allowed to settle
// on section headers (Enter zooms), project sub-headers (Enter scopes), and
// task rows. Separators are visual-only and must never be selected after a
// rebuild, filter, collapse, paging, or mode change.
func (m *OutlinerModel) ClampCursor() {
	m.ClampCursorTo(len(m.Rows))
	if len(m.Rows) == 0 {
		return
	}
	if m.Rows[m.Cursor].IsNavigable() {
		return
	}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		if m.Rows[i].IsNavigable() {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.Rows[i].IsNavigable() {
			m.Cursor = i
			return
		}
	}
}

// contentHeight returns the number of task rows visible on screen after the
// one-row operational title, the one-row menu bar above it when it is shown,
// and the context row below it when some axis is live. Scope and mode are part
// of that title and context is exactly one further row, so neither ever
// consumes a third breadcrumb.
//
// This is the one definition of the row budget, and every caller wants the
// reduced value: the window, the page size, and the suggestion budgets are all
// measured against the list the chrome leaves behind.
func (m *OutlinerModel) ContentHeight() int {
	height := m.Height() - 1 - m.menuBarRows() - m.contextRowRows()
	if height < 1 {
		return 1
	}
	return height
}
