package tui

// isInTextInput returns true if the current focus is on a text input field.
func (m *WorkflowRuleCreatorModel) isInTextInput() bool {
	section, _, fieldIdx := m.getFocusSection()
	switch section {
	case "name", "priority":
		return true
	case "trigger":
		return fieldIdx == 2
	case "action":
		return fieldIdx == 1 || fieldIdx == 4
	case "scope":
		return true
	}

	return false
}

// blurCurrentField removes focus from the currently focused field.
func (m *WorkflowRuleCreatorModel) blurCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Blur()
	case "priority":
		m.priorityInput.Blur()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Blur()
		}
	case "action":
		if itemIdx < len(m.actions) {
			switch fieldIdx {
			case 1:
				m.actions[itemIdx].valueInput.Blur()
			case 4:
				m.actions[itemIdx].condValueInput.Blur()
			}
		}
	case "scope":
		switch fieldIdx {
		case 0:
			m.scopeProjectsInput.Blur()
		case 1:
			m.scopeTypesInput.Blur()
		case 2:
			m.scopeStatesInput.Blur()
		}
	}
}

// focusCurrentField sets focus on the currently focused field.
func (m *WorkflowRuleCreatorModel) focusCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Focus()
	case "priority":
		m.priorityInput.Focus()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Focus()
		}
	case "action":
		if itemIdx < len(m.actions) {
			switch fieldIdx {
			case 1:
				m.actions[itemIdx].valueInput.Focus()
			case 4:
				m.actions[itemIdx].condValueInput.Focus()
			}
		}
	case "scope":
		switch fieldIdx {
		case 0:
			m.scopeProjectsInput.Focus()
		case 1:
			m.scopeTypesInput.Focus()
		case 2:
			m.scopeStatesInput.Focus()
		}
	}
}

// focusFieldCount returns total number of focusable fields for the current tab.
func (m *WorkflowRuleCreatorModel) focusFieldCount() int {
	count := 2
	switch m.activeTab {
	case tabTriggers:
		count += len(m.triggers) * 3
		if len(m.triggers) > 1 {
			count++
		}
	case tabActions:
		for _, action := range m.actions {
			count += 2
			if action.hasCondition {
				count += 3
			}
		}
	case tabScope:
		count += 3
	}

	return count
}

// getFocusSection returns the section, item index, and field index for the current focus.
func (m *WorkflowRuleCreatorModel) getFocusSection() (section string, itemIdx int, fieldIdx int) {
	idx := m.focusIndex
	if idx == 0 {
		return "name", 0, 0
	}
	idx--

	if idx == 0 {
		return "priority", 0, 0
	}
	idx--

	switch m.activeTab {
	case tabTriggers:
		triggersTotal := len(m.triggers) * 3
		if idx < triggersTotal {
			return "trigger", idx / 3, idx % 3
		}
		idx -= triggersTotal
		if len(m.triggers) > 1 && idx == 0 {
			return "logic", 0, 0
		}
	case tabActions:
		for i, action := range m.actions {
			fieldsForAction := 2
			if action.hasCondition {
				fieldsForAction = 5
			}
			if idx < fieldsForAction {
				return "action", i, idx
			}
			idx -= fieldsForAction
		}
	case tabScope:
		if idx < 3 {
			return "scope", 0, idx
		}
	}

	return "name", 0, 0
}

// actionFieldOffset returns the cumulative focus offset for the given action index.
func (m *WorkflowRuleCreatorModel) actionFieldOffset(actionIdx int) int {
	offset := 0
	for i := 0; i < actionIdx && i < len(m.actions); i++ {
		offset += 2
		if m.actions[i].hasCondition {
			offset += 3
		}
	}

	return offset
}

// updateScrollForFocus adjusts scroll offset to keep the focused item visible.
func (m *WorkflowRuleCreatorModel) updateScrollForFocus() {
	section, itemIdx, fieldIdx := m.getFocusSection()

	var focusLine int
	switch section {
	case "name":
		focusLine = 0
	case "priority":
		focusLine = 2
	case "trigger":
		focusLine = 6 + itemIdx*4
	case "logic":
		focusLine = 6 + len(m.triggers)*4
	case "action":
		focusLine = 6 + itemIdx*3
	case "scope":
		focusLine = 6 + fieldIdx*3
	}

	visibleHeight := max(m.height-7, 5)
	if focusLine < m.editorScrollOffset {
		m.editorScrollOffset = focusLine
	} else if focusLine >= m.editorScrollOffset+visibleHeight-2 {
		m.editorScrollOffset = focusLine - visibleHeight + 3
	}

	if m.editorScrollOffset < 0 {
		m.editorScrollOffset = 0
	}
}
