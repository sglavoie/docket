package tui

import tea "charm.land/bubbletea/v2"

// typeChangeOpForKey maps a key to a TypeChangeOp, or returns false if not a type key.
func typeChangeOpForKey(key string) (TypeChangeOp, string, bool) {
	switch key {
	case "<":
		return TypePromote, "Promote Task", true
	case ">":
		return TypeDemote, "Demote Task", true
	case "H":
		return TypeCycleLeft, "Change Type", true
	case "L":
		return TypeCycleRight, "Change Type", true
	}
	return 0, "", false
}

// handleTypeKeys handles <, >, H, L - type change operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTypeKeys(key string) (bool, tea.Cmd) {
	cmd, handled := handleTypeChangeKey(key, m)
	return handled, cmd
}

// handleMovementKeys handles K, J, ←, → - task movement operations.
// ←/→ handle indent/outdent, h/l are used for section switching.
// Captures task states before changes to enable event-based workflow triggers.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMovementKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "K":
		// Move task up (swap with sibling above)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			visibleSiblings := m.getVisibleSiblings(task)
			visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
			if visibleSiblingIdx > 0 {
				siblingAbove := visibleSiblings[visibleSiblingIdx-1]
				if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot move completed task above uncompleted task"}
					}
				}
				if m.viewMode == ViewModeActive && task.Active != siblingAbove.Active {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot reorder across active/non-active boundary"}
					}
				}
				prevStates := captureTaskStates(m.project.Tasks)
				m.project.SwapTasks(task, siblingAbove)
				if m.cursor > 0 {
					m.cursor--
				}
				m.setUndoContext("Move Up", task.Title)
				return true, m.saveAndRefreshWithPrevStates(prevStates)
			}
		}
		return true, nil
	case "J":
		// Move task down (swap with sibling below)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			visibleSiblings := m.getVisibleSiblings(task)
			visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
			if visibleSiblingIdx >= 0 && visibleSiblingIdx < len(visibleSiblings)-1 {
				siblingBelow := visibleSiblings[visibleSiblingIdx+1]
				if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot move uncompleted task below completed task"}
					}
				}
				if m.viewMode == ViewModeActive && task.Active != siblingBelow.Active {
					return true, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot reorder across active/non-active boundary"}
					}
				}
				prevStates := captureTaskStates(m.project.Tasks)
				m.project.SwapTasks(task, siblingBelow)
				if m.cursor < len(visible)-1 {
					m.cursor++
				}
				m.setUndoContext("Move Down", task.Title)
				return true, m.saveAndRefreshWithPrevStates(prevStates)
			}
		}
		return true, nil
	case "right":
		// Indent task (move right in hierarchy, nest under task above)
		if CursorInBounds(m.cursor, len(visible)) && m.cursor > 0 {
			task := visible[m.cursor].Task
			candidate := visible[m.cursor-1].Task
			prevStates := captureTaskStates(m.project.Tasks)
			if IndentTask(m.project, task, candidate) {
				m.setUndoContext("Indent", task.Title)
				return true, m.saveAndRefreshWithPrevStates(prevStates)
			}
		}
		return true, nil
	case "left":
		// Outdent task (move left in hierarchy, to parent's level)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			prevStates := captureTaskStates(m.project.Tasks)
			if OutdentTask(m.project, task) {
				m.setUndoContext("Outdent", task.Title)
				return true, m.saveAndRefreshWithPrevStates(prevStates)
			}
		}
		return true, nil
	}
	return false, nil
}

// moveTaskToTop moves the current task to the top of its visible siblings
// by repeatedly swapping up, matching the proven K (swap-up) mechanism.
func (m *ProjectTreeModel) moveTaskToTop(visible []*TreeNode) (tea.Model, tea.Cmd) {
	if !CursorInBounds(m.cursor, len(visible)) {
		return m, nil
	}
	task := visible[m.cursor].Task
	visibleSiblings := m.getVisibleSiblings(task)
	visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
	if visibleSiblingIdx <= 0 {
		return m, nil // Already at top or not found
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// Swap up through each visible sibling, same as repeated K presses
	for i := visibleSiblingIdx - 1; i >= 0; i-- {
		target := visibleSiblings[i]
		if task.IsRoot() && task.IsComplete() && !target.IsComplete() {
			break
		}
		m.project.SwapTasks(task, target)
	}

	// Position cursor at the first sibling's old location
	for i, node := range visible {
		if node.Task.ID == visibleSiblings[0].ID {
			m.cursor = i
			break
		}
	}
	m.setUndoContext("Move to Top", task.Title)
	return m, m.saveAndRefreshWithPrevStates(prevStates)
}

// moveTaskToBottom moves the current task to the bottom of its visible siblings
// by repeatedly swapping down, matching the proven J (swap-down) mechanism.
func (m *ProjectTreeModel) moveTaskToBottom(visible []*TreeNode) (tea.Model, tea.Cmd) {
	if !CursorInBounds(m.cursor, len(visible)) {
		return m, nil
	}
	task := visible[m.cursor].Task
	visibleSiblings := m.getVisibleSiblings(task)
	visibleSiblingIdx := m.findVisibleSiblingIndex(task, visibleSiblings)
	if visibleSiblingIdx == -1 || visibleSiblingIdx >= len(visibleSiblings)-1 {
		return m, nil // Already at bottom or not found
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// Swap down through each visible sibling, same as repeated J presses
	for i := visibleSiblingIdx + 1; i < len(visibleSiblings); i++ {
		target := visibleSiblings[i]
		if task.IsRoot() && !task.IsComplete() && target.IsComplete() {
			break
		}
		m.project.SwapTasks(task, target)
	}

	// Position cursor at the last sibling's old location
	for i, node := range visible {
		if node.Task.ID == visibleSiblings[len(visibleSiblings)-1].ID {
			m.cursor = i
			break
		}
	}
	m.setUndoContext("Move to Bottom", task.Title)
	return m, m.saveAndRefreshWithPrevStates(prevStates)
}
