package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// loadTasks populates the Tasks map by filtering project tasks into columns.
// Only non-completed tasks are included. Each task is placed in the column
// matching its Section field (empty string maps to Active column).
// Filters are applied to determine which tasks are visible.
// After filtering, hierarchical nodes are built for each column.
func (m *KanbanViewModel) loadTasks() {
	// Delegate to swimlane loader when swimlanes are active
	if m.swimlaneEnabled() {
		m.loadTasksWithSwimlanes()
		return
	}

	// Reset custom section row focus if no custom sections exist
	if m.customSectionTabs() == nil {
		m.customSectionRowFocused = false
		m.customSectionTab = 0
	}

	// Clear existing tasks and nodes
	for _, def := range m.columnDefs {
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	if m.Project == nil {
		return
	}

	// Pass 1: Filter and categorize tasks by column
	for _, task := range m.Project.Tasks {
		// Handle completed tasks based on group-by mode
		if task.Status.IsComplete() {
			// In status mode, allow recently completed tasks through
			if m.groupBy == GroupByStatus && IsRecentlyCompleted(task, CompletedInStatusModeDays) {
				// Allow — will be placed in Done/Canceled column below
			} else {
				continue
			}
		}

		// Skip snoozed tasks
		if task.IsSnoozed() {
			continue
		}

		// Apply filters - skip tasks that don't pass
		if !m.passesFilters(task) {
			continue
		}

		// In focus mode, only include focused task and its descendants
		if m.FocusedTask != nil && !m.isInFocusedSubtree(task) {
			continue
		}

		// Determine which column this task belongs to
		colKey := m.taskColumnKey(task)
		// Validate the column key is one of our known columns
		if _, exists := m.Columns[colKey]; !exists {
			if IsSectionGroupBy(m.groupBy) {
				// Unknown section defaults to Active (empty string)
				colKey = KanbanSectionActive
			} else {
				// For other modes, skip tasks with unknown column keys
				continue
			}
		}

		m.Tasks[colKey] = append(m.Tasks[colKey], task)
	}

	// Pass 2: Build hierarchical nodes for each column
	for _, def := range m.columnDefs {
		m.buildColumnNodes(def.Key)
		// Apply sorting after building nodes
		m.sortColumnNodes(def.Key)
	}

	// Pass 3: Compute ChildrenElsewhere counts
	m.computeChildrenElsewhere()

	// Update node visibility based on expand state
	m.updateNodeVisibility()

	// Validate cursor positions for each column after loading
	for _, def := range m.columnDefs {
		visibleCount := m.visibleNodeCount(def.Key)
		m.Columns[def.Key].Validate(visibleCount)
	}

	// Calculate filter counts for the filter bar
	m.calculateFilterCounts()
}

// isInFocusedSubtree checks if a task is the focused task or one of its descendants.
func (m *KanbanViewModel) isInFocusedSubtree(task *models.Task) bool {
	if m.FocusedTask == nil {
		return true
	}
	if task.ID == m.FocusedTask.ID {
		return true
	}
	// Walk up the parent chain
	current := task
	for current.Parent != nil {
		if current.Parent.ID == m.FocusedTask.ID {
			return true
		}
		current = current.Parent
	}
	return false
}

// buildColumnNodes creates hierarchical KanbanNode entries for a section.
// Root tasks (no parent or parent not in section) appear at level 0.
// Children are nested below their parents with increasing levels.
func (m *KanbanViewModel) buildColumnNodes(section string) {
	tasks := m.Tasks[section]
	if len(tasks) == 0 {
		return
	}

	// Build a set of task IDs in this section for quick lookup
	taskIDsInSection := make(map[string]bool)
	for _, task := range tasks {
		taskIDsInSection[task.ID] = true
	}

	// Identify root tasks: no parent, or parent not in this section
	var rootTasks []*models.Task
	for _, task := range tasks {
		isRoot := task.ParentID == nil || !taskIDsInSection[*task.ParentID]
		// In focus mode, the focused task is always a root
		if m.FocusedTask != nil && task.ID == m.FocusedTask.ID {
			isRoot = true
		}
		if isRoot {
			rootTasks = append(rootTasks, task)
		}
	}

	// Add nodes recursively starting from root tasks
	for _, root := range rootTasks {
		m.addKanbanNode(root, 0, section, taskIDsInSection)
	}
}

// addKanbanNode recursively adds a task and its children to the Nodes list.
func (m *KanbanViewModel) addKanbanNode(task *models.Task, level int, section string, taskIDsInSection map[string]bool) {
	// Cap level at MaxKanbanDisplayLevel
	displayLevel := level
	if displayLevel > MaxKanbanDisplayLevel {
		displayLevel = MaxKanbanDisplayLevel
	}

	// Check if this task has children in the same section
	hasChildrenHere := false
	for _, child := range task.Children {
		if taskIDsInSection[child.ID] {
			hasChildrenHere = true
			break
		}
	}

	// Get or default expand state (default = expanded)
	expanded := true
	if state, ok := m.ExpandState[task.ID]; ok {
		expanded = state
	} else {
		m.ExpandState[task.ID] = true
	}

	node := &KanbanNode{
		Task:            task,
		Level:           displayLevel,
		Expanded:        expanded,
		Visible:         true, // Will be updated in updateNodeVisibility
		HasChildrenHere: hasChildrenHere,
	}
	m.Nodes[section] = append(m.Nodes[section], node)

	// Recursively add children that are in this section
	for _, child := range task.Children {
		if taskIDsInSection[child.ID] {
			m.addKanbanNode(child, level+1, section, taskIDsInSection)
		}
	}
}

// computeChildrenElsewhere calculates how many children are in other columns.
func (m *KanbanViewModel) computeChildrenElsewhere() {
	// Build a map of which section each task is in
	taskSections := make(map[string]string)
	for section, tasks := range m.Tasks {
		for _, task := range tasks {
			taskSections[task.ID] = section
		}
	}

	// For each node, count children in other columns
	for _, def := range m.columnDefs {
		section := def.Key
		for _, node := range m.Nodes[section] {
			count := 0
			for _, child := range node.Task.Children {
				// Skip completed children
				if child.Status.IsComplete() {
					continue
				}
				childSection, inView := taskSections[child.ID]
				if inView && childSection != section {
					count++
				}
			}
			node.ChildrenElsewhere = count
		}
	}
}

// updateNodeVisibility sets Visible based on parent expand state.
func (m *KanbanViewModel) updateNodeVisibility() {
	for _, def := range m.columnDefs {
		section := def.Key
		nodes := m.Nodes[section]
		for i, node := range nodes {
			// Root nodes (level 0) are always visible
			if node.Level == 0 {
				node.Visible = true
				continue
			}

			// Find parent node (must be earlier in list with lower level)
			parentVisible := false
			parentExpanded := false
			for j := i - 1; j >= 0; j-- {
				if nodes[j].Level < node.Level {
					parentVisible = nodes[j].Visible
					parentExpanded = nodes[j].Expanded
					break
				}
			}

			// Visible if parent is visible AND expanded
			node.Visible = parentVisible && parentExpanded
		}
	}
}

// visibleNodeCount returns the count of visible nodes in a section.
func (m *KanbanViewModel) visibleNodeCount(section string) int {
	count := 0
	for _, node := range m.Nodes[section] {
		if node.Visible {
			count++
		}
	}
	return count
}

// visibleNodes returns only visible nodes for a section.
func (m *KanbanViewModel) visibleNodesForSection(section string) []*KanbanNode {
	var visible []*KanbanNode
	for _, node := range m.Nodes[section] {
		if node.Visible {
			visible = append(visible, node)
		}
	}
	return visible
}

// CurrentSection returns the key of the currently active column.
// In section mode this is the section name; in other modes it's the column key.
func (m *KanbanViewModel) CurrentSection() string {
	return m.activeColumnKey()
}

// CurrentColumnTasks returns the tasks in the currently active column.
func (m *KanbanViewModel) CurrentColumnTasks() []*models.Task {
	return m.Tasks[m.activeColumnKey()]
}

// TasksForSection returns the tasks in the specified section.
func (m *KanbanViewModel) TasksForSection(section string) []*models.Task {
	if tasks, exists := m.Tasks[section]; exists {
		return tasks
	}
	return nil
}
