package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// sortColumnNodes sorts the nodes in a column based on the current sort settings.
// Sorting is applied within each hierarchy level to preserve parent-child relationships.
func (m *KanbanViewModel) sortColumnNodes(section string) {
	// Get the sort field for this column (use column-specific or global)
	sortField := m.getSortFieldForColumn(section)
	if sortField == SortByDefault {
		return
	}

	nodes := m.Nodes[section]
	if len(nodes) <= 1 {
		return
	}

	// Group nodes by their parent within this section
	nodesByParent := make(map[string][]*KanbanNode)
	var rootNodes []*KanbanNode

	// Build a set of task IDs in this section
	taskIDsInSection := make(map[string]bool)
	for _, node := range nodes {
		taskIDsInSection[node.Task.ID] = true
	}

	for _, node := range nodes {
		if node.Task.ParentID == nil || !taskIDsInSection[*node.Task.ParentID] {
			rootNodes = append(rootNodes, node)
		} else {
			parentID := *node.Task.ParentID
			nodesByParent[parentID] = append(nodesByParent[parentID], node)
		}
	}

	// Sort root nodes
	m.sortKanbanNodeSlice(rootNodes, sortField)

	// Rebuild nodes slice in sorted order
	m.Nodes[section] = []*KanbanNode{}
	var addNodeWithChildren func(node *KanbanNode)
	addNodeWithChildren = func(node *KanbanNode) {
		m.Nodes[section] = append(m.Nodes[section], node)
		children := nodesByParent[node.Task.ID]
		m.sortKanbanNodeSlice(children, sortField)
		for _, child := range children {
			addNodeWithChildren(child)
		}
	}

	for _, root := range rootNodes {
		addNodeWithChildren(root)
	}
}

// getSortFieldForColumn returns the sort field for a specific column.
// Returns column-specific sort if set, otherwise the global sort field.
func (m *KanbanViewModel) getSortFieldForColumn(section string) SortField {
	if field, ok := m.columnSort[section]; ok {
		return field
	}
	return m.sortField
}

// setSortFieldForColumn sets the sort field for a specific column.
func (m *KanbanViewModel) setSortFieldForColumn(section string, field SortField) {
	if field == SortByDefault {
		delete(m.columnSort, section)
	} else {
		m.columnSort[section] = field
	}
}

// sortKanbanNodeSlice sorts a slice of kanban nodes based on the sort field.
func (m *KanbanViewModel) sortKanbanNodeSlice(nodes []*KanbanNode, sortField SortField) {
	if len(nodes) <= 1 {
		return
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return m.compareKanbanNodes(nodes[i], nodes[j], sortField)
	})
}

// compareKanbanNodes compares two kanban nodes based on the sort field and direction.
func (m *KanbanViewModel) compareKanbanNodes(a, b *KanbanNode, sortField SortField) bool {
	var less bool

	switch sortField {
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

	default:
		less = false
	}

	if !m.sortAscending {
		less = !less
	}

	return less
}

// statusOrder returns the sort order for a status (workflow order).
func (m *KanbanViewModel) statusOrder(status models.TaskStatus) int {
	switch status {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusBlocked:
		return 2
	case models.StatusDone:
		return 3
	case models.StatusCanceled:
		return 4
	default:
		return 5
	}
}

// cycleSortField cycles the sort field for the current column.
// If pressing the same number, it toggles the sort direction.
// If pressing a different number, it switches to that sort field (ascending).
func (m *KanbanViewModel) cycleSortField(newField SortField) {
	section := m.CurrentSection()
	currentField := m.getSortFieldForColumn(section)

	if currentField == newField {
		// Same field - toggle direction
		m.sortAscending = !m.sortAscending
	} else {
		// Different field - switch to it (ascending)
		m.setSortFieldForColumn(section, newField)
		m.sortAscending = true
	}

	// Re-sort and update
	m.sortColumnNodes(section)
	m.updateNodeVisibility()

	// Update status message
	direction := "↑"
	if !m.sortAscending {
		direction = "↓"
	}
	m.statusMessage = "Sort: " + newField.String() + " " + direction
}

// resetSort resets sorting to default for the current column.
func (m *KanbanViewModel) resetSort() {
	section := m.CurrentSection()
	m.setSortFieldForColumn(section, SortByDefault)
	m.sortAscending = true
	m.loadTasks()
	m.statusMessage = "Sort reset to default"
}
