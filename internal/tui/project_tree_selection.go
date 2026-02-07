package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// toggleSelection toggles the selection state of a single task.
func (m *ProjectTreeModel) toggleSelection(task *models.Task) {
	if m.selectedTasks[task.ID] {
		delete(m.selectedTasks, task.ID)
	} else {
		m.selectedTasks[task.ID] = true
	}
}

// clearSelection exits multi-select mode and clears all selections.
func (m *ProjectTreeModel) clearSelection() {
	m.multiSelectMode = false
	m.selectedTasks = make(map[string]bool)
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
	var less bool

	switch m.sortField {
	case SortByPriority:
		// Lower priority number = higher priority (1 is highest)
		less = a.Task.Priority < b.Task.Priority

	case SortByDueDate:
		// nil due dates go to the end
		if a.Task.DueDate == nil && b.Task.DueDate == nil {
			less = false
		} else if a.Task.DueDate == nil {
			less = false // a goes after b
		} else if b.Task.DueDate == nil {
			less = true // a goes before b
		} else {
			less = a.Task.DueDate.Before(*b.Task.DueDate)
		}

	case SortByCreatedAt:
		less = a.Task.CreatedAt.Before(b.Task.CreatedAt)

	case SortByStatus:
		less = m.statusOrder(a.Task.Status) < m.statusOrder(b.Task.Status)

	case SortByScore:
		profile := m.config.GetProjectScoringProfile(m.project.ID)
		if profile != nil {
			scoreA := scoring.ComputeScore(a.Task, profile)
			scoreB := scoring.ComputeScore(b.Task, profile)
			less = scoreA > scoreB // higher score first (descending by default)
		}

	default:
		// In snoozed view with default sort, sort by SnoozedUntil ascending
		if m.viewMode == ViewModeSnoozed {
			if a.Task.SnoozedUntil == nil && b.Task.SnoozedUntil == nil {
				less = false
			} else if a.Task.SnoozedUntil == nil {
				less = false
			} else if b.Task.SnoozedUntil == nil {
				less = true
			} else {
				less = a.Task.SnoozedUntil.Before(*b.Task.SnoozedUntil)
			}
		} else {
			less = false
		}
	}

	if !m.sortAscending {
		less = !less
	}

	return less
}

// statusOrder returns the sort order for a status (workflow order).
func (m *ProjectTreeModel) statusOrder(status models.TaskStatus) int {
	switch status {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusReview:
		return 2
	case models.StatusBlocked:
		return 3
	case models.StatusDone:
		return 4
	case models.StatusCanceled:
		return 5
	default:
		return 6
	}
}
