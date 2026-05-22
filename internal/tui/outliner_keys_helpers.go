package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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

// sendTaskToSection moves the cursor task to the specified section.
func (m *OutlinerModel) sendTaskToSection(task *models.Task, section string) (tea.Model, tea.Cmd) {
	if task == nil {
		return m, nil
	}
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.SetSection(section)
		if section == "" {
			t.Activate()
		}
	})
	m.project.RelocateTaskToEndOfSection(task.ID, section)
	label := "Move to " + section
	if section == "" {
		label = "Move to Active"
	}
	m.setUndoContext(label, task.Title)
	m.undo.TaskIDs = []string{task.ID}
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// sendTasksToSectionBulk moves all selected tasks to the specified section.
func (m *OutlinerModel) sendTasksToSectionBulk(section string) (tea.Model, tea.Cmd) {
	count := 0
	for id := range m.selectedTasks {
		if m.project.MutateTask(id, func(t *models.Task) {
			t.SetSection(section)
			if section == "" {
				t.Activate()
			}
		}) {
			m.project.RelocateTaskToEndOfSection(id, section)
			count++
		}
	}
	label := "Move to " + section
	if section == "" {
		label = "Move to Active"
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
	return m, m.saveWithWorkflow()
}

// crossMoveTaskToPosition handles moveTaskToPosition in cross-project mode.
func (m *OutlinerModel) crossMoveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	row, proj, ok := m.crossGetMutationTarget()
	if !ok || row.task.ID != task.ID {
		return m, nil
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
			proj.SwapTasks(task, siblings[i])
		}
	} else {
		for i := idx + 1; i < len(siblings); i++ {
			if task.IsRoot() && !task.IsComplete() && siblings[i].IsComplete() {
				break
			}
			proj.SwapTasks(task, siblings[i])
		}
	}
	label := "Move to Top"
	if !toTop {
		label = "Move to Bottom"
	}
	m.crossUndoCtx.Set(label, task.Title)
	m.crossUndoCtx.ProjectIDs = []string{proj.ID}
	m.rebuildRows()
	m.moveCursorToTask(task.ID)
	return m, m.crossSaveProject(proj)
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

// visibleSiblings returns the contiguous run of sibling rows in m.rows around
// cursorIdx — tasks sharing the same parent ID and ProjectID as the cursor row,
// uninterrupted by section headers, separators, or rows with a different parent.
// Returns nil when cursorIdx is out of range or points at a header/separator/nil
// task row. The contiguous-run semantics confine J/K reorders to a single
// section bucket (and to a single project in cross-project mode) without
// dereferencing potentially stale cloned-parent Children slices.
func (m *OutlinerModel) visibleSiblings(cursorIdx int) []*models.Task {
	if cursorIdx < 0 || cursorIdx >= len(m.rows) {
		return nil
	}
	cursorRow := m.rows[cursorIdx]
	if cursorRow.IsSectionHeader || cursorRow.IsSeparator || cursorRow.task == nil {
		return nil
	}
	cursorParentID := parentIDOf(cursorRow.task)
	cursorProjectID := cursorRow.ProjectID

	inRun := func(row outlinerRow) bool {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
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
		if !inRun(m.rows[i]) {
			break
		}
		leftSide = append(leftSide, m.rows[i].task)
	}
	// Reverse leftSide so siblings appear in row order.
	for i, j := 0, len(leftSide)-1; i < j; i, j = i+1, j-1 {
		leftSide[i], leftSide[j] = leftSide[j], leftSide[i]
	}

	siblings := append(leftSide, cursorRow.task)
	for i := cursorIdx + 1; i < len(m.rows); i++ {
		if !inRun(m.rows[i]) {
			break
		}
		siblings = append(siblings, m.rows[i].task)
	}
	return siblings
}

// moveCursorToTask positions the cursor on the row matching the given task ID.
// Falls back to clamping if the task is not found in the current rows.
func (m *OutlinerModel) moveCursorToTask(taskID string) {
	for i, row := range m.rows {
		if !row.IsSectionHeader && row.task != nil && row.task.ID == taskID {
			m.cursor = i
			return
		}
	}
	m.clampCursor()
}
