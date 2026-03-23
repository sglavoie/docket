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

// collapseAllParents marks all parent tasks as collapsed recursively.
func (m *OutlinerModel) collapseAllParents(tasks []*models.Task) {
	for _, task := range tasks {
		if len(task.Children) > 0 {
			m.expandedNodes[task.ID] = false
			m.collapseAllParents(task.Children)
		}
	}
}

// sendTaskToSection moves the cursor task to the specified section.
func (m *OutlinerModel) sendTaskToSection(task *models.Task, section string) (tea.Model, tea.Cmd) {
	if task == nil {
		return m, nil
	}
	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Section = section
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
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// sendTasksToSectionBulk moves all selected tasks to the specified section.
func (m *OutlinerModel) sendTasksToSectionBulk(section string) (tea.Model, tea.Cmd) {
	count := 0
	for id := range m.selectedTasks {
		if m.project.MutateTask(id, func(t *models.Task) {
			t.Section = section
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
	m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
	m.clearMultiSelect()
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// moveTaskToPosition moves the cursor task to the top or bottom of its siblings.
func (m *OutlinerModel) moveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	if m.project == nil {
		return m.crossMoveTaskToPosition(task, toTop)
	}
	siblings := m.getSiblings(task, m.project)
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
			m.project.SwapTasks(task, siblings[i])
		}
	} else {
		for i := idx + 1; i < len(siblings); i++ {
			m.project.SwapTasks(task, siblings[i])
		}
	}

	label := "Move to Top"
	if !toTop {
		label = "Move to Bottom"
	}
	m.setUndoContext(label, task.Title)
	m.rebuildRows()
	return m, m.saveWithWorkflow()
}

// crossMoveTaskToPosition handles moveTaskToPosition in cross-project mode.
func (m *OutlinerModel) crossMoveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	row, proj, ok := m.crossGetMutationTarget()
	if !ok || row.task.ID != task.ID {
		return m, nil
	}
	siblings := m.getSiblings(task, proj)
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
			proj.SwapTasks(task, siblings[i])
		}
	} else {
		for i := idx + 1; i < len(siblings); i++ {
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

// getSiblings returns the sibling list for a task at the current zoom level.
// proj must be non-nil; callers in cross-project mode resolve it from the row.
func (m *OutlinerModel) getSiblings(task *models.Task, proj *models.Project) []*models.Task {
	if proj == nil {
		return nil
	}
	if task.Parent != nil {
		return task.Parent.Children
	}
	// Root level or zoomed level
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if t := proj.GetTaskByID(topID); t != nil {
			return t.Children
		}
	}
	return proj.GetRootTasks()
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
