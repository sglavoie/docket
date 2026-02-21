package tui

import (
	"sort"
)

// sortColumnNodes sorts the nodes in a column based on the current sort settings.
// Sorting is applied within each hierarchy level to preserve parent-child relationships.
func (m *KanbanViewModel) sortColumnNodes(section string) {
	// Get the sort field for this column (use column-specific or global)
	sortField := m.getSortFieldForColumn(section)
	if sortField == SortByDefault && section != KanbanSectionActive {
		return
	}

	nodes := m.Nodes[section]
	if len(nodes) <= 1 {
		return
	}

	// Build a set of task IDs in this section so nodes whose parent is in
	// another section are treated as roots.
	taskIDsInSection := make(map[string]bool)
	for _, node := range nodes {
		taskIDsInSection[node.Task.ID] = true
	}

	m.Nodes[section] = sortHierarchical(
		nodes,
		func(n *KanbanNode) string { return n.Task.ID },
		func(n *KanbanNode) string {
			if n.Task.ParentID == nil || !taskIDsInSection[*n.Task.ParentID] {
				return ""
			}
			return *n.Task.ParentID
		},
		func(s []*KanbanNode) { m.sortKanbanNodeSlice(s, sortField, section) },
	)
}

// getSortFieldForColumn returns the sort field for a specific column.
// Returns column-specific sort if set, otherwise the global sort field.
func (m *KanbanViewModel) getSortFieldForColumn(section string) SortField {
	if field, ok := m.columnSort[section]; ok {
		return field
	}
	return m.sort.Field
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
func (m *KanbanViewModel) sortKanbanNodeSlice(nodes []*KanbanNode, sortField SortField, section string) {
	if len(nodes) <= 1 {
		return
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return m.compareKanbanNodes(nodes[i], nodes[j], sortField, section)
	})
}

// compareKanbanNodes compares two kanban nodes based on the sort field and direction.
func (m *KanbanViewModel) compareKanbanNodes(a, b *KanbanNode, sortField SortField, section string) bool {
	// In the Active section, float active-flagged tasks to the top
	if section == KanbanSectionActive && a.Task.Active != b.Task.Active {
		return a.Task.Active
	}

	if sortField == SortByDefault {
		return false
	}

	cmp := CompareTasks(a.Task, b.Task, sortField, m.Project.ID, m.config)
	return ApplySortDirection(cmp, m.sort.Ascending)
}

// cycleSortField cycles the sort field for the current column.
// If pressing the same number, it toggles the sort direction.
// If pressing a different number, it switches to that sort field (ascending).
func (m *KanbanViewModel) cycleSortField(newField SortField) {
	section := m.CurrentSection()
	currentField := m.getSortFieldForColumn(section)

	if currentField == newField {
		// Same field - toggle direction
		m.sort.Ascending = !m.sort.Ascending
	} else {
		// Different field - switch to it (ascending)
		m.setSortFieldForColumn(section, newField)
		m.sort.Ascending = true
	}

	// Re-sort and update
	m.sortColumnNodes(section)
	m.updateNodeVisibility()

	// Update status message
	direction := "↑"
	if !m.sort.Ascending {
		direction = "↓"
	}
	m.statusMessage = "Sort: " + newField.String() + " " + direction
}

// resetSort resets sorting to default for the current column.
func (m *KanbanViewModel) resetSort() {
	section := m.CurrentSection()
	m.setSortFieldForColumn(section, SortByDefault)
	m.sort.Ascending = true
	m.loadTasks()
	m.statusMessage = "Sort reset to default"
}
