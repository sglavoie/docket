package tui

import tea "github.com/charmbracelet/bubbletea"

// handleTypeKeys handles h, l, H, L - type change operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTypeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "h":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanPromote(); ok {
				task.Promote()
				m.setUndoContext("Promote Task", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "l":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanDemote(); ok {
				task.Demote()
				m.setUndoContext("Demote Task", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "H":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanCycleLeafType(true); ok {
				task.CycleLeafType(true)
				m.setUndoContext("Change Type", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "L":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanCycleLeafType(false); ok {
				task.CycleLeafType(false)
				m.setUndoContext("Change Type", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	}
	return false, nil
}

// handleMovementKeys handles K, J, left, right - task movement operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMovementKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "K":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			visibleSiblings := m.getVisibleSiblings(task)
			visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
			if visibleSiblingIdx > 0 {
				siblingAbove := visibleSiblings[visibleSiblingIdx-1]
				if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "Cannot move completed task above uncompleted task"}
					}
				}
				m.project.SwapTasks(task, siblingAbove)
				if m.cursor > 0 {
					m.cursor--
				}
				m.setUndoContext("Move Up", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "J":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			visibleSiblings := m.getVisibleSiblings(task)
			visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
			if visibleSiblingIdx >= 0 && visibleSiblingIdx < len(visibleSiblings)-1 {
				siblingBelow := visibleSiblings[visibleSiblingIdx+1]
				if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "Cannot move uncompleted task below completed task"}
					}
				}
				m.project.SwapTasks(task, siblingBelow)
				if m.cursor < len(visible)-1 {
					m.cursor++
				}
				m.setUndoContext("Move Down", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	case "right":
		if CursorInBounds(m.cursor, len(visible)) && m.cursor > 0 {
			task := visible[m.cursor].Task
			// Walk up from the visually-above task to find a valid parent
			candidate := visible[m.cursor-1].Task
			for candidate != nil {
				if ok, _ := task.CanIndent(candidate); ok {
					task.ParentID = &candidate.ID
					m.project.BuildTaskTree()
					m.setUndoContext("Indent", task.Title)
					return true, m.saveAndRefresh()
				}
				candidate = candidate.Parent
			}
		}
		return true, nil
	case "left":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if ok, _ := task.CanOutdent(); ok {
				m.project.OutdentTask(task)
				m.setUndoContext("Outdent", task.Title)
				return true, m.saveAndRefresh()
			}
		}
		return true, nil
	}
	return false, nil
}
