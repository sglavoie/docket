package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// toggleSelection toggles the selection state of a single task.
func (m *ProjectTreeModel) toggleSelection(task *models.Task) {
	m.ToggleSelection(task.ID)
}

// clearSelection exits multi-select mode and clears all selections.
func (m *ProjectTreeModel) clearSelection() {
	m.multiSelectMode = false
	m.ClearSelection()
}

// getSelectedTask returns the task under the cursor, or nil if none.
func (m *ProjectTreeModel) getSelectedTask() *models.Task {
	visible := m.visibleNodes()
	if CursorInBounds(m.cursor, len(visible)) {
		return visible[m.cursor].Task
	}
	return nil
}

// getSelectedNode returns the tree node under the cursor, or nil if none.
func (m *ProjectTreeModel) getSelectedNode() *TreeNode {
	visible := m.visibleNodes()
	if CursorInBounds(m.cursor, len(visible)) {
		return visible[m.cursor]
	}
	return nil
}

// getSelectedTasks returns a slice of selected tasks from the project.
func (m *ProjectTreeModel) getSelectedTasks() []*models.Task {
	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		if m.selectedTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// selectAllVisible adds all currently visible tasks to the selection.
func (m *ProjectTreeModel) selectAllVisible() {
	for _, node := range m.visibleNodes() {
		m.selectedTasks[node.Task.ID] = true
	}
}

// deselectAllVisible removes all currently visible tasks from the selection.
func (m *ProjectTreeModel) deselectAllVisible() {
	for _, node := range m.visibleNodes() {
		delete(m.selectedTasks, node.Task.ID)
	}
}

// pruneInvisibleSelections removes selected tasks that are no longer visible.
func (m *ProjectTreeModel) pruneInvisibleSelections() {
	if !m.multiSelectMode {
		return
	}
	visibleIDs := make(map[string]bool)
	for _, node := range m.visibleNodes() {
		visibleIDs[node.Task.ID] = true
	}
	for id := range m.selectedTasks {
		if !visibleIDs[id] {
			delete(m.selectedTasks, id)
		}
	}
}

// sortNodes sorts the nodes based on the current sort field.
// Sorting is applied within each hierarchy level to preserve parent-child relationships.
func (m *ProjectTreeModel) sortNodes() {
	if m.sortField == SortByDefault && m.viewMode != ViewModeSnoozed {
		return
	}

	// Group nodes by their parent
	nodesByParent := make(map[string][]*TreeNode)
	var rootNodes []*TreeNode

	for _, node := range m.nodes {
		if node.Task.ParentID == nil {
			rootNodes = append(rootNodes, node)
		} else {
			parentID := *node.Task.ParentID
			nodesByParent[parentID] = append(nodesByParent[parentID], node)
		}
	}

	// Sort root nodes
	m.sortNodeSlice(rootNodes)

	// Rebuild the nodes slice in sorted order
	m.nodes = []*TreeNode{}
	var addNodeWithChildren func(node *TreeNode)
	addNodeWithChildren = func(node *TreeNode) {
		m.nodes = append(m.nodes, node)
		children := nodesByParent[node.Task.ID]
		m.sortNodeSlice(children)
		for _, child := range children {
			addNodeWithChildren(child)
		}
	}

	for _, root := range rootNodes {
		addNodeWithChildren(root)
	}
}

// sortNodeSlice sorts a slice of nodes based on current sort settings.
func (m *ProjectTreeModel) sortNodeSlice(nodes []*TreeNode) {
	if len(nodes) <= 1 {
		return
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return m.compareNodes(nodes[i], nodes[j])
	})
}

// compareNodes compares two nodes based on the current sort field and direction.
func (m *ProjectTreeModel) compareNodes(a, b *TreeNode) bool {
	// Handle snoozed view default sort specially
	if m.sortField == SortByDefault && m.viewMode == ViewModeSnoozed {
		var less bool
		if a.Task.SnoozedUntil == nil && b.Task.SnoozedUntil == nil {
			less = false
		} else if a.Task.SnoozedUntil == nil {
			less = false
		} else if b.Task.SnoozedUntil == nil {
			less = true
		} else {
			less = a.Task.SnoozedUntil.Before(*b.Task.SnoozedUntil)
		}
		if !m.sortAscending {
			less = !less
		}
		return less
	}

	cmp := CompareTasks(a.Task, b.Task, m.sortField, m.project.ID, m.config)
	return ApplySortDirection(cmp, m.sortAscending)
}
