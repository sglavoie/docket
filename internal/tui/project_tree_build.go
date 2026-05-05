package tui

import "github.com/sglavoie/tt/internal/models"

func (m *ProjectTreeModel) buildTree() {
	if m.project == nil {
		return
	}

	// Preserve expanded state before rebuilding
	expandedState := make(map[string]bool)
	for _, node := range m.nodes {
		expandedState[node.Task.ID] = node.Expanded
	}

	m.nodes = []*TreeNode{}

	// Get root tasks - either the focused task or actual root tasks
	var rootTasks []*models.Task
	if m.focusedTask != nil {
		// In focus mode, the focused task is the only "root"
		rootTasks = []*models.Task{m.focusedTask}
	} else {
		rootTasks = m.project.GetRootTasks()
	}

	// Build nodes recursively
	for i, task := range rootTasks {
		isLast := i == len(rootTasks)-1
		m.addTaskNode(task, 0, expandedState, nil, isLast)
	}

	m.sortNodes()
	m.updateVisibility()
	m.calculateFilterCounts()
}

func (m *ProjectTreeModel) addTaskNode(task *models.Task, level int, expandedState map[string]bool, ancestorIsLast []bool, isLast bool) {
	// Use preserved state if available, otherwise default to expanded
	// This ensures newly-parented tasks (that just gained children) show their children
	expanded, hasState := expandedState[task.ID]
	if !hasState {
		expanded = true
	}

	node := &TreeNode{
		Task:       task,
		Level:      level,
		Expanded:   expanded,
		Visible:    true,
		TreePrefix: buildTreePrefix(ancestorIsLast, isLast),
	}
	m.nodes = append(m.nodes, node)

	// Add children, copying the ancestor stack to prevent slice aliasing
	if len(task.Children) > 0 {
		childAncestors := make([]bool, len(ancestorIsLast)+1)
		copy(childAncestors, ancestorIsLast)
		childAncestors[len(ancestorIsLast)] = isLast
		for i, child := range task.Children {
			isChildLast := i == len(task.Children)-1
			m.addTaskNode(child, level+1, expandedState, childAncestors, isChildLast)
		}
	}
}

func (m *ProjectTreeModel) visibleNodes() []*TreeNode {
	var visible []*TreeNode
	for _, node := range m.nodes {
		if node.Visible {
			visible = append(visible, node)
		}
	}
	return visible
}

// findNodeByTaskID returns the TreeNode for the given task ID, or nil if not found.
func (m *ProjectTreeModel) findNodeByTaskID(id string) *TreeNode {
	for _, node := range m.nodes {
		if node.Task.ID == id {
			return node
		}
	}
	return nil
}
