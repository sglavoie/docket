package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/workflow"
)

// Update handles state transitions for the workflow rule creator.
func (m *WorkflowRuleCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle picker overlays first
	if m.showingFieldPicker && m.fieldPicker != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.fieldPicker.SetSize(msg.Width, msg.Height)
			return m, nil
		case TriggerFieldPickerDoneMsg:
			m.showingFieldPicker = false
			if !msg.Canceled && m.fieldPickerTarget < len(m.triggers) {
				// Find cursor position for selected field
				for i, f := range m.triggerFields {
					if f == msg.Field {
						m.triggers[m.fieldPickerTarget].fieldCursor = i
						break
					}
				}
			}
			m.fieldPicker = nil
			return m, nil
		default:
			_, cmd = m.fieldPicker.Update(msg)
			return m, cmd
		}
	}

	if m.showingActionPicker && m.actionPicker != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.actionPicker.SetSize(msg.Width, msg.Height)
			return m, nil
		case ActionTypePickerDoneMsg:
			m.showingActionPicker = false
			if !msg.Canceled && m.actionPickerTarget < len(m.actions) {
				// Find cursor position for selected action type
				for i, at := range m.actionTypes {
					if at == msg.ActionType {
						m.actions[m.actionPickerTarget].typeCursor = i
						break
					}
				}
			}
			m.actionPicker = nil
			return m, nil
		default:
			_, cmd = m.actionPicker.Update(msg)
			return m, cmd
		}
	}

	// Handle discard confirmation
	if m.confirmDiscard {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return BackMsg{}
				}
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// If showing preview on narrow screen, go back to editor
			if m.previewShown && !m.isWideScreen() {
				m.previewShown = false
				return m, nil
			}
			// Check if form has content
			hasContent := strings.TrimSpace(m.nameInput.Value()) != "" || len(m.triggers) > 1 || len(m.actions) > 1
			if !hasContent {
				// Check if the single trigger or action has a value filled in
				for _, t := range m.triggers {
					if strings.TrimSpace(t.valueInput.Value()) != "" {
						hasContent = true
						break
					}
				}
				if !hasContent {
					for _, a := range m.actions {
						if strings.TrimSpace(a.valueInput.Value()) != "" {
							hasContent = true
							break
						}
					}
				}
			}
			if hasContent {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "[":
			// Switch to previous tab
			if m.activeTab > tabTriggers {
				m.blurCurrentField()
				m.activeTab--
				m.focusIndex = 0
				m.focusCurrentField()
			}
			return m, nil

		case "]":
			// Switch to next tab
			if m.activeTab < tabScope {
				m.blurCurrentField()
				m.activeTab++
				m.focusIndex = 0
				m.focusCurrentField()
			}
			return m, nil

		case "ctrl+p":
			// Toggle preview pane (narrow screens only)
			if !m.isWideScreen() {
				m.previewShown = !m.previewShown
			}
			return m, nil

		case "ctrl+f":
			// Switch to triggers tab and open field picker for first trigger
			m.blurCurrentField()
			m.activeTab = tabTriggers
			m.focusIndex = 2 // First trigger's field selector
			m.focusCurrentField()

			// Open field picker
			targetIdx := 0
			if targetIdx < len(m.triggers) {
				currentField := m.triggerFields[m.triggers[targetIdx].fieldCursor]
				m.fieldPicker = NewTriggerFieldPickerModel(targetIdx, currentField)
				m.fieldPicker.SetSize(m.width, m.height)
				m.fieldPickerTarget = targetIdx
				m.showingFieldPicker = true
			}
			return m, nil

		case "ctrl+t":
			// Switch to actions tab and open type picker for first action
			m.blurCurrentField()
			m.activeTab = tabActions
			m.focusIndex = 2 // First action's type selector
			m.focusCurrentField()

			// Open action type picker
			targetIdx := 0
			if targetIdx < len(m.actions) {
				currentType := m.actionTypes[m.actions[targetIdx].typeCursor]
				m.actionPicker = NewActionTypePickerModel(targetIdx, currentType)
				m.actionPicker.SetSize(m.width, m.height)
				m.actionPickerTarget = targetIdx
				m.showingActionPicker = true
			}
			return m, nil

		case "tab", "shift+tab":
			m.blurCurrentField()
			total := m.focusFieldCount()
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % total
			} else {
				m.focusIndex = (m.focusIndex + total - 1) % total
			}
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field
			m.blurCurrentField()
			total := m.focusFieldCount()
			m.focusIndex = (m.focusIndex + 1) % total
			m.focusCurrentField()
			return m, nil

		case "ctrl+s":
			return m, m.saveRule()

		case "ctrl+r":
			// Test current rule via dry run
			rule, err := m.buildRuleFromForm()
			if err != nil {
				m.err = err
				return m, nil
			}
			return m, func() tea.Msg {
				return ShowWorkflowDryRunMsg{SingleRule: rule}
			}

		case "+", "=":
			return m, m.handleAdd()

		case "-", "_":
			return m, m.handleRemove()

		case "c":
			// Toggle condition on current action (only on actions tab, when focused on action)
			if m.activeTab == tabActions && !m.isInTextInput() {
				section, itemIdx, _ := m.getFocusSection()
				if section == "action" && itemIdx < len(m.actions) {
					m.actions[itemIdx].hasCondition = !m.actions[itemIdx].hasCondition
				}
			}
			return m, nil

		case "g":
			// Toggle trigger grouping (only on triggers tab, when focused on trigger)
			if m.activeTab == tabTriggers && !m.isInTextInput() {
				section, itemIdx, _ := m.getFocusSection()
				if section == "trigger" && itemIdx < len(m.triggers) {
					m.toggleTriggerGroup(itemIdx)
				}
			}
			return m, nil

		case " ":
			section, _, _ := m.getFocusSection()
			if section == "logic" {
				if m.triggerLogic == workflow.LogicAnd {
					m.triggerLogic = workflow.LogicOr
				} else {
					m.triggerLogic = workflow.LogicAnd
				}
				return m, nil
			}

		case "left":
			return m, m.handleLeft()

		case "right":
			return m, m.handleRight()

		case "up":
			return m, m.handleUp()

		case "down":
			return m, m.handleDown()

		case "h":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleLeft()
			}

		case "l":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleRight()
			}

		case "k":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleUp()
			}

		case "j":
			// Only handle as navigation if not in a text input field
			if !m.isInTextInput() {
				return m, m.handleDown()
			}
		}
	}

	// Update focused text input
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput, cmd = m.nameInput.Update(msg)
	case "priority":
		m.priorityInput, cmd = m.priorityInput.Update(msg)
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput, cmd = m.triggers[itemIdx].valueInput.Update(msg)
		}
	case "action":
		if itemIdx < len(m.actions) {
			switch fieldIdx {
			case 1:
				m.actions[itemIdx].valueInput, cmd = m.actions[itemIdx].valueInput.Update(msg)
			case 4: // condValue
				m.actions[itemIdx].condValueInput, cmd = m.actions[itemIdx].condValueInput.Update(msg)
			}
		}
	case "scope":
		switch fieldIdx {
		case 0:
			m.scopeProjectsInput, cmd = m.scopeProjectsInput.Update(msg)
		case 1:
			m.scopeTypesInput, cmd = m.scopeTypesInput.Update(msg)
		case 2:
			m.scopeSectionsInput, cmd = m.scopeSectionsInput.Update(msg)
		}
	}

	return m, cmd
}

// isInTextInput returns true if the current focus is on a text input field.
func (m *WorkflowRuleCreatorModel) isInTextInput() bool {
	section, _, fieldIdx := m.getFocusSection()
	switch section {
	case "name", "priority":
		return true
	case "trigger":
		return fieldIdx == 2 // value input
	case "action":
		return fieldIdx == 1 || fieldIdx == 4 // value input or condValue input
	case "scope":
		return true // all scope fields are text inputs
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
			case 4: // condValue
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
			m.scopeSectionsInput.Blur()
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
			case 4: // condValue
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
			m.scopeSectionsInput.Focus()
		}
	}
}

// handleLeft moves cursor left within selectors.
func (m *WorkflowRuleCreatorModel) handleLeft() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "trigger":
		if itemIdx < len(m.triggers) {
			switch fieldIdx {
			case 0: // field selector
				if m.triggers[itemIdx].fieldCursor > 0 {
					m.triggers[itemIdx].fieldCursor--
				}
			case 1: // op selector
				if m.triggers[itemIdx].opCursor > 0 {
					m.triggers[itemIdx].opCursor--
				}
			}
		}
	case "logic":
		if m.triggerLogic == workflow.LogicOr {
			m.triggerLogic = workflow.LogicAnd
		}
	case "action":
		if itemIdx < len(m.actions) {
			switch fieldIdx {
			case 0: // type selector
				if m.actions[itemIdx].typeCursor > 0 {
					m.actions[itemIdx].typeCursor--
				}
			case 2: // condField selector
				if m.actions[itemIdx].condFieldCursor > 0 {
					m.actions[itemIdx].condFieldCursor--
				}
			case 3: // condOp selector
				if m.actions[itemIdx].condOpCursor > 0 {
					m.actions[itemIdx].condOpCursor--
				}
			}
		}
	}
	return nil
}

// handleRight moves cursor right within selectors.
func (m *WorkflowRuleCreatorModel) handleRight() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "trigger":
		if itemIdx < len(m.triggers) {
			switch fieldIdx {
			case 0: // field selector
				if m.triggers[itemIdx].fieldCursor < len(m.triggerFields)-1 {
					m.triggers[itemIdx].fieldCursor++
				}
			case 1: // op selector
				if m.triggers[itemIdx].opCursor < len(m.triggerOps)-1 {
					m.triggers[itemIdx].opCursor++
				}
			}
		}
	case "logic":
		if m.triggerLogic == workflow.LogicAnd {
			m.triggerLogic = workflow.LogicOr
		}
	case "action":
		if itemIdx < len(m.actions) {
			switch fieldIdx {
			case 0: // type selector
				if m.actions[itemIdx].typeCursor < len(m.actionTypes)-1 {
					m.actions[itemIdx].typeCursor++
				}
			case 2: // condField selector
				if m.actions[itemIdx].condFieldCursor < len(m.triggerFields)-1 {
					m.actions[itemIdx].condFieldCursor++
				}
			case 3: // condOp selector
				if m.actions[itemIdx].condOpCursor < len(m.triggerOps)-1 {
					m.actions[itemIdx].condOpCursor++
				}
			}
		}
	}
	return nil
}

// handleUp moves to the previous item (trigger or action).
func (m *WorkflowRuleCreatorModel) handleUp() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
		// Already at top, wrap to last item
		switch m.activeTab {
		case tabTriggers:
			if len(m.triggers) > 1 {
				m.focusIndex = 2 + len(m.triggers)*3
			} else {
				m.focusIndex = 2
			}
		case tabActions:
			m.focusIndex = 2 + m.actionFieldOffset(len(m.actions)-1)
		case tabScope:
			m.focusIndex = 4 // last scope field (sections)
		}
	case "priority":
		m.focusIndex = 0
	case "trigger":
		if itemIdx > 0 {
			m.focusIndex = 2 + (itemIdx-1)*3
		} else {
			m.focusIndex = 1 // priority
		}
	case "logic":
		m.focusIndex = 2 + (len(m.triggers)-1)*3
	case "action":
		if itemIdx > 0 {
			m.focusIndex = 2 + m.actionFieldOffset(itemIdx-1)
		} else {
			m.focusIndex = 1 // priority
		}
	case "scope":
		if fieldIdx > 0 {
			m.focusIndex = fieldIdx + 1 // move up within scope fields
		} else {
			m.focusIndex = 1 // priority
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// handleDown moves to the next item (trigger or action).
func (m *WorkflowRuleCreatorModel) handleDown() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
		m.focusIndex = 1 // priority
	case "priority":
		m.focusIndex = 2 // first item in current tab
	case "trigger":
		if itemIdx < len(m.triggers)-1 {
			m.focusIndex = 2 + (itemIdx+1)*3
		} else if len(m.triggers) > 1 {
			m.focusIndex = 2 + len(m.triggers)*3
		} else {
			m.focusIndex = 0
		}
	case "logic":
		m.focusIndex = 0
	case "action":
		if itemIdx < len(m.actions)-1 {
			m.focusIndex = 2 + m.actionFieldOffset(itemIdx+1)
		} else {
			m.focusIndex = 0
		}
	case "scope":
		if fieldIdx < 2 {
			m.focusIndex = fieldIdx + 3 // move down within scope fields
		} else {
			m.focusIndex = 0
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// updateScrollForFocus adjusts scroll offset to keep focused item visible.
func (m *WorkflowRuleCreatorModel) updateScrollForFocus() {
	section, itemIdx, fieldIdx := m.getFocusSection()

	// Estimate line position of focused item
	// Name takes ~2 lines, section header ~2 lines
	// Each trigger takes ~4 lines, each action ~3 lines
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

	// Keep focused item visible with some margin
	visibleHeight := m.height - 10 // Approximate visible area
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	if focusLine < m.editorScrollOffset {
		m.editorScrollOffset = focusLine
	} else if focusLine >= m.editorScrollOffset+visibleHeight-2 {
		m.editorScrollOffset = focusLine - visibleHeight + 3
	}

	if m.editorScrollOffset < 0 {
		m.editorScrollOffset = 0
	}
}

// actionFieldOffset returns the cumulative focus offset for the given action index,
// accounting for variable field counts (2 base + 3 if hasCondition).
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

// handleAdd adds a new trigger or action based on current tab.
func (m *WorkflowRuleCreatorModel) handleAdd() tea.Cmd {
	factory := DefaultFormFieldFactory()

	if m.activeTab == tabTriggers {
		// Add a new trigger
		valueInput := factory.NewTextInput("Value")
		m.triggers = append(m.triggers, TriggerFormState{valueInput: valueInput, groupIndex: -1})
	} else {
		// Add a new action
		valueInput := factory.NewTextInput("Value")
		condValueInput := factory.NewTextInput("Condition value")
		m.actions = append(m.actions, ActionFormState{valueInput: valueInput, condValueInput: condValueInput})
	}
	return nil
}

// handleRemove removes the current trigger or action based on current tab.
func (m *WorkflowRuleCreatorModel) handleRemove() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()

	if m.activeTab == tabTriggers {
		// Remove trigger (must have at least one)
		if section == "trigger" && len(m.triggers) > 1 && itemIdx < len(m.triggers) {
			m.triggers = append(m.triggers[:itemIdx], m.triggers[itemIdx+1:]...)
			// Adjust focus if needed
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
	} else {
		// Remove action (must have at least one)
		if section == "action" && len(m.actions) > 1 && itemIdx < len(m.actions) {
			m.actions = append(m.actions[:itemIdx], m.actions[itemIdx+1:]...)
			// Adjust focus if needed
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
	}
	return nil
}

// toggleTriggerGroup toggles a trigger in/out of a sub-group.
// If the trigger is ungrouped, it joins or creates a group.
// If the trigger is in a group, it is removed from the group.
func (m *WorkflowRuleCreatorModel) toggleTriggerGroup(triggerIdx int) {
	t := &m.triggers[triggerIdx]
	if t.groupIndex >= 0 {
		// Remove from group
		oldGroup := t.groupIndex
		t.groupIndex = -1
		// Clean up empty groups
		m.cleanupEmptyGroups(oldGroup)
	} else {
		// Add to the first existing group, or create a new one
		if len(m.triggerGroups) == 0 {
			m.triggerGroups = append(m.triggerGroups, TriggerGroupFormState{logic: workflow.LogicAnd})
		}
		t.groupIndex = 0
	}
}

// cleanupEmptyGroups removes a trigger group if it has fewer than 2 members,
// and reassigns remaining triggers.
func (m *WorkflowRuleCreatorModel) cleanupEmptyGroups(groupIdx int) {
	// Count members of this group
	count := 0
	for _, t := range m.triggers {
		if t.groupIndex == groupIdx {
			count++
		}
	}
	if count >= 2 {
		return // group still has enough members
	}

	// Ungroup any remaining single member
	for i := range m.triggers {
		if m.triggers[i].groupIndex == groupIdx {
			m.triggers[i].groupIndex = -1
		}
	}

	// Remove the group and re-index
	m.triggerGroups = append(m.triggerGroups[:groupIdx], m.triggerGroups[groupIdx+1:]...)
	for i := range m.triggers {
		if m.triggers[i].groupIndex > groupIdx {
			m.triggers[i].groupIndex--
		}
	}
}
