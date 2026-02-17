package tui

// toggleCurrentExpand toggles the expand state of the selected node.
func (m *KanbanViewModel) toggleCurrentExpand() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = !node.Expanded
	m.updateNodeVisibility()
	// Validate cursor after visibility change
	section := m.CurrentSection()
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// expandCurrent expands the selected node.
func (m *KanbanViewModel) expandCurrent() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = true
	m.updateNodeVisibility()
}

// collapseCurrent collapses the selected node.
func (m *KanbanViewModel) collapseCurrent() {
	node := m.getSelectedNode()
	if node == nil || !node.HasChildrenHere {
		return
	}
	m.ExpandState[node.Task.ID] = false
	m.updateNodeVisibility()
	// Validate cursor after visibility change
	section := m.CurrentSection()
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// expandAllInColumn expands all nodes in the current column.
func (m *KanbanViewModel) expandAllInColumn() {
	section := m.CurrentSection()
	for _, node := range m.Nodes[section] {
		if node.HasChildrenHere {
			m.ExpandState[node.Task.ID] = true
		}
	}
	m.updateNodeVisibility()
}

// collapseAllInColumn collapses all nodes in the current column.
func (m *KanbanViewModel) collapseAllInColumn() {
	section := m.CurrentSection()
	for _, node := range m.Nodes[section] {
		if node.HasChildrenHere {
			m.ExpandState[node.Task.ID] = false
		}
	}
	m.updateNodeVisibility()
	// Validate cursor after collapsing
	m.Columns[section].Validate(m.visibleNodeCount(section))
}

// focusSubtree enters focus mode, showing only the selected task and its descendants.
func (m *KanbanViewModel) focusSubtree() {
	task := m.getSelectedTask()
	if task == nil {
		return
	}
	m.FocusedTask = task
	m.loadTasks()
	m.statusMessage = "Focus mode: " + task.Title
}

// exitFocusMode exits focus mode and shows all tasks.
func (m *KanbanViewModel) exitFocusMode() {
	m.FocusedTask = nil
	m.loadTasks()
	m.statusMessage = ""
}
