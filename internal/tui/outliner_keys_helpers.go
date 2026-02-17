package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.expandedNodes) == 0 {
		m.expandedNodes = make(map[string]bool)
		m.markAllExpanded(m.project.GetRootTasks())
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

// moveTaskToPosition moves the cursor task to the top or bottom of its siblings.
func (m *OutlinerModel) moveTaskToPosition(task *models.Task, toTop bool) (tea.Model, tea.Cmd) {
	siblings := m.getSiblings(task)
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

// getSiblings returns the sibling list for a task at the current zoom level.
func (m *OutlinerModel) getSiblings(task *models.Task) []*models.Task {
	if task.Parent != nil {
		return task.Parent.Children
	}
	// Root level or zoomed level
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if t := m.project.GetTaskByID(topID); t != nil {
			return t.Children
		}
	}
	return m.project.GetRootTasks()
}
