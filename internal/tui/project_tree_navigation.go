package tui

import "github.com/sglavoie/tt/internal/models"

// cyclableViewModes returns the view modes that h/l should cycle through.
// Snoozed is included only when the project has snoozed tasks.
func (m *ProjectTreeModel) cyclableViewModes() []ViewMode {
	modes := []ViewMode{ViewModeActive, ViewModeNext, ViewModeInbox, ViewModeWaiting, ViewModeSomeday}
	if m.project == nil {
		return modes
	}
	for _, task := range m.project.Tasks {
		if task.IsSnoozed() {
			return append(modes, ViewModeSnoozed)
		}
	}
	return modes
}

func viewModeIndex(modes []ViewMode, mode ViewMode) int {
	for i, m := range modes {
		if m == mode {
			return i
		}
	}
	return -1
}

// cycleViewMode cycles the view mode forward (delta=+1) or backward (delta=-1)
// through the cyclable modes, then refreshes visibility and clamps the cursor.
func (m *ProjectTreeModel) cycleViewMode(delta int) {
	modes := m.cyclableViewModes()
	if len(modes) == 0 {
		return
	}
	if idx := viewModeIndex(modes, m.viewMode); idx >= 0 {
		m.viewMode = modes[(idx+delta+len(modes))%len(modes)]
	} else if delta > 0 {
		m.viewMode = modes[0]
	} else {
		m.viewMode = modes[len(modes)-1]
	}
	m.updateVisibility()
	m.clampCursor()
}

// rootNodeIndices returns the indices of root-level (Level == 0) nodes
// in the visible list, for use with navigateToNextSection/navigateToPrevSection.
func rootNodeIndices(visible []*TreeNode) []int {
	var indices []int
	for i, node := range visible {
		if node.Level == 0 {
			indices = append(indices, i)
		}
	}
	return indices
}

// JumpToTask moves the cursor to the specified task in the visible list.
func (m *ProjectTreeModel) JumpToTask(task *models.Task) {
	if task == nil {
		return
	}
	m.buildTree()
	visible := m.visibleNodes()
	for i, node := range visible {
		if node.Task.ID == task.ID {
			m.cursor = i
			return
		}
	}
}
