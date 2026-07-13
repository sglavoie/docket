package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Standard status messages for outliner keybindings that need a task
// or owning project under the cursor. Centralized so cross-project and
// single-project paths emit identical text.
const (
	statusNoTaskSelected       = "No task selected"
	statusNoProjectUnderCursor = "No project under cursor"
)

// setNoTaskStatus shows the standard "No task selected" message used by
// outliner keybindings that require a cursor task (header, separator,
// or empty row under the cursor).
func (m *OutlinerModel) setNoTaskStatus() {
	m.setStatus(statusNoTaskSelected)
}

// setNoProjectStatus shows the standard "No project under cursor"
// message used by outliner keybindings that need an owning project but
// cannot resolve one (cross-project header/separator, or stale project
// ID).
func (m *OutlinerModel) setNoProjectStatus() {
	m.setStatus(statusNoProjectUnderCursor)
}

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.expandedNodes) == 0 {
		m.expandedNodes = make(map[string]bool)
		if m.project != nil {
			m.markAllExpanded(m.project.GetRootTasks())
		} else {
			for _, p := range m.loadedProjects {
				m.markAllExpanded(p.GetRootTasks())
			}
		}
	}
}

// markAllExpanded marks all parent tasks as expanded recursively.
func (m *OutlinerModel) markAllExpanded(tasks []*models.Task) {
	for _, task := range tasks {
		if len(task.Children) > 0 {
			m.expandedNodes[task.ID] = true
			m.markAllExpanded(task.Children)
		}
	}
}

// collapseToDepth expands nodes at depth < N and collapses nodes at depth >= N.
func (m *OutlinerModel) collapseToDepth(depth int) {
	m.ensureExpandedNodes()
	var walk func(tasks []*models.Task)
	walk = func(tasks []*models.Task) {
		for _, task := range tasks {
			if len(task.Children) > 0 {
				m.expandedNodes[task.ID] = task.Level < depth
				walk(task.Children)
			}
		}
	}
	if m.project != nil {
		walk(m.project.GetRootTasks())
	} else {
		for _, p := range m.loadedProjects {
			walk(p.GetRootTasks())
		}
	}
	m.rebuildRows()
}

// sendTaskToSection moves the cursor task to the specified section through the
// canonical single-task coordinator (app Service.SetSection). This is the
// finalized retired-section decision: the coordinator writes only the GTD
// section field (resyncing the derived workflow state); the legacy overlay
// composition — reviving the retired Active flag when the section is empty and
// RelocateTaskToEndOfSection cosmetic reordering — is intentionally NOT applied.
// The coordinator commits one ledger-backed transaction and the shell owns
// reload + toast. Mirrors the cross-project crossSendTaskToSection.
func (m *OutlinerModel) sendTaskToSection(task *models.Task, section string) (tea.Model, tea.Cmd) {
	if task == nil {
		return m, nil
	}
	return m, m.taskItem.SetSection(task.ID, section)
}

// sendTasksToSectionBulk moves all selected tasks to the specified section.
// It applies the canonical Task.SetSection to each selected task and commits
// once atomically — the retired Active flag and RelocateTaskToEndOfSection
// cosmetic reorder are intentionally NOT applied, matching sendTaskToSection's
// finalized retired-section behavior. This multi-task path stays on the atomic
// workflow save (there is no bulk SetSection coordinator seam; looping the
// single-task coordinator would commit each task as a separate last-write-wins
// transaction and lose updates); the coordinator migration for outliner bulk
// section is deferred to the later bulk-owning child.
func (m *OutlinerModel) sendTasksToSectionBulk(section string) (tea.Model, tea.Cmd) {
	count := 0
	for id := range m.selectedTasks {
		if m.project.MutateTask(id, func(t *models.Task) {
			t.SetSection(section)
		}) {
			count++
		}
	}
	label := "Move to " + section
	if section == "" {
		label = "Move to Ready"
	}
	selectedIDs := make([]string, 0, len(m.selectedTasks))
	for id := range m.selectedTasks {
		selectedIDs = append(selectedIDs, id)
	}
	m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
	m.undo.TaskIDs = selectedIDs
	m.clearMultiSelect()
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// moveTaskToPosition moves the cursor task to the top or bottom of its siblings.
func (m *OutlinerModel) moveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	if m.project == nil {
		return m.crossMoveTaskToPosition(task, toTop)
	}
	siblings := m.visibleSiblings(m.cursor)
	idx := -1
	for i, s := range siblings {
		if s.ID == task.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return m, nil
	}

	if toTop && idx == 0 {
		return m, nil
	}
	if !toTop && idx == len(siblings)-1 {
		return m, nil
	}

	if toTop {
		for i := idx - 1; i >= 0; i-- {
			if task.IsRoot() && task.IsComplete() && !siblings[i].IsComplete() {
				break
			}
			m.project.SwapTasks(task, siblings[i])
		}
	} else {
		for i := idx + 1; i < len(siblings); i++ {
			if task.IsRoot() && !task.IsComplete() && siblings[i].IsComplete() {
				break
			}
			m.project.SwapTasks(task, siblings[i])
		}
	}

	label := "Move to Top"
	if !toTop {
		label = "Move to Bottom"
	}
	m.setUndoContext(label, task.Title)
	m.undo.TaskIDs = []string{task.ID}
	m.rebuildRows()
	m.moveCursorToTask(task.ID)
	return m, m.saveWithWorkflow()
}

// crossMoveTaskToPosition handles moveTaskToPosition in cross-project mode by
// delegating to the app-layer MoveToTop/MoveToBottom seams. Those bubble the task
// to the edge of its MODEL-order siblings (honoring the same root completion
// boundary) and commit one ledger transaction; the shell reloads the cross
// outliner. This is the documented VISIBLE-vs-MODEL ordering decision for the
// cross-project structural mutations: at the sibling edge, model-order and the
// legacy visible-order converge on the same terminal position, so move-to-top /
// move-to-bottom keep their user-visible outcome while unifying on the app layer's
// single source of truth.
func (m *OutlinerModel) crossMoveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	row, _, ok := m.crossGetMutationTarget()
	if !ok || row.task.ID != task.ID {
		return m, nil
	}
	if toTop {
		return m, m.taskItem.MoveToTop(task.ID)
	}
	return m, m.taskItem.MoveToBottom(task.ID)
}

// jumpToNextSection moves the cursor to the next (direction=1) or previous
// (direction=-1) section header, cycling through the list.
func (m *OutlinerModel) jumpToNextSection(direction int) tea.Cmd {
	if len(m.rows) == 0 {
		return nil
	}

	// Collect section header indices
	var sectionIndices []int
	for i, row := range m.rows {
		if row.IsSectionHeader {
			sectionIndices = append(sectionIndices, i)
		}
	}
	if len(sectionIndices) == 0 {
		return nil
	}

	old := m.cursor
	if direction > 0 {
		// Find the next section header after the cursor
		found := false
		for _, idx := range sectionIndices {
			if idx > m.cursor {
				m.cursor = idx
				found = true
				break
			}
		}
		if !found {
			// Wrap to first section
			m.cursor = sectionIndices[0]
		}
	} else {
		// Find the previous section header before the cursor
		found := false
		for i := len(sectionIndices) - 1; i >= 0; i-- {
			if sectionIndices[i] < m.cursor {
				m.cursor = sectionIndices[i]
				found = true
				break
			}
		}
		if !found {
			// Wrap to last section
			m.cursor = sectionIndices[len(sectionIndices)-1]
		}
	}
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

// parentIDOf returns the parent task ID for t, or "" if t is a root.
func parentIDOf(t *models.Task) string {
	if t == nil || t.ParentID == nil {
		return ""
	}
	return *t.ParentID
}

// visibleSiblings returns the visible sibling rows in m.rows around cursorIdx —
// tasks sharing the same parent ID and ProjectID as the cursor row, gathered
// across a window bounded by section headers, project headers, and separators.
// Non-sibling task rows inside the window (e.g. expanded descendants of a
// sibling) are skipped rather than treated as hard stops, so J/K reorders and
// move-to-top/bottom can target siblings on either side of an expanded parent.
// Returns nil when cursorIdx is out of range or points at a header/separator/nil
// task row. The window boundaries confine reorders to a single section bucket
// (and to a single project in cross-project mode) without dereferencing
// potentially stale cloned-parent Children slices.
func (m *OutlinerModel) visibleSiblings(cursorIdx int) []*models.Task {
	if cursorIdx < 0 || cursorIdx >= len(m.rows) {
		return nil
	}
	cursorRow := m.rows[cursorIdx]
	if !cursorRow.isTaskRow() || cursorRow.task == nil {
		return nil
	}
	cursorParentID := parentIDOf(cursorRow.task)
	cursorProjectID := cursorRow.ProjectID

	// isBoundary reports whether a row terminates the reorder window. Section
	// headers, project sub-headers, and separators delimit buckets; a task row
	// in a different project also bounds the window in cross-project mode.
	isBoundary := func(row outlinerRow) bool {
		if !row.isTaskRow() {
			return true
		}
		if row.task != nil && row.ProjectID != cursorProjectID {
			return true
		}
		return false
	}

	// isSibling reports whether a task row shares the cursor's parent (and
	// project). Non-matching task rows inside the window are skipped, not stops.
	isSibling := func(row outlinerRow) bool {
		if !row.isTaskRow() || row.task == nil {
			return false
		}
		if parentIDOf(row.task) != cursorParentID {
			return false
		}
		if row.ProjectID != cursorProjectID {
			return false
		}
		return true
	}

	var leftSide []*models.Task
	for i := cursorIdx - 1; i >= 0; i-- {
		if isBoundary(m.rows[i]) {
			break
		}
		if isSibling(m.rows[i]) {
			leftSide = append(leftSide, m.rows[i].task)
		}
	}
	// Reverse leftSide so siblings appear in row order.
	for i, j := 0, len(leftSide)-1; i < j; i, j = i+1, j-1 {
		leftSide[i], leftSide[j] = leftSide[j], leftSide[i]
	}

	siblings := append(leftSide, cursorRow.task)
	for i := cursorIdx + 1; i < len(m.rows); i++ {
		if isBoundary(m.rows[i]) {
			break
		}
		if isSibling(m.rows[i]) {
			siblings = append(siblings, m.rows[i].task)
		}
	}
	return siblings
}

// moveCursorToTask positions the cursor on the row matching the given task ID.
// Falls back to clamping if the task is not found in the current rows.
func (m *OutlinerModel) moveCursorToTask(taskID string) {
	for i, row := range m.rows {
		if row.isTaskRow() && row.task != nil && row.task.ID == taskID {
			m.cursor = i
			return
		}
	}
	m.clampCursor()
}
