package tui

import tea "github.com/charmbracelet/bubbletea"

// handleNavigationKeys handles j/k, g/G, arrow keys, and page navigation.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleNavigationKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return true, nil
	case "down", "j":
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
		return true, nil
	case "g":
		m.cursor = 0
		return true, nil
	case "G":
		if len(visible) > 0 {
			m.cursor = len(visible) - 1
		}
		return true, nil
	}
	return false, nil
}

// handleSelectionKeys handles v, a, space - selection operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSelectionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case " ":
		if m.multiSelectMode {
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			node := visible[m.cursor]
			if len(node.Task.Children) > 0 {
				node.Expanded = !node.Expanded
				m.updateVisibility()
			}
		}
		return true, nil
	case "tab":
		// Tab now toggles sidebar (Space still does expand/collapse)
		m.sidebarEnabled = !m.sidebarEnabled
		return true, nil
	case "v":
		if m.multiSelectMode {
			m.clearSelection()
		} else {
			m.multiSelectMode = true
			if CursorInBounds(m.cursor, len(visible)) {
				m.toggleSelection(visible[m.cursor].Task)
			}
		}
		return true, nil
	case "a":
		if m.multiSelectMode {
			m.selectAllVisible()
			return true, nil
		}
		return false, nil
	case "A":
		if m.multiSelectMode {
			for _, node := range m.visibleNodes() {
				delete(m.selectedTasks, node.Task.ID)
			}
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

// handleFocusKeys handles f - focus mode operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFocusKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	if key == "f" {
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			// Only allow focus on tasks that can have children (epics, stories)
			if task.Type.CanHaveChildren() {
				m.focusedTask = task
				m.cursor = 0 // Reset cursor to top
				m.buildTree()
			}
		}
		return true, nil
	}
	return false, nil
}

// handleArchiveKey handles A - archive/unarchive project (requires special handling for multi-select deselect).
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleArchiveKey(visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode {
		for _, node := range m.visibleNodes() {
			delete(m.selectedTasks, node.Task.ID)
		}
		return true, nil
	}
	if m.project.Archived {
		m.confirmAction = "unarchive"
	} else {
		m.confirmAction = "archive"
	}
	return true, nil
}
