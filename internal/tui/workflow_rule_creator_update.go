package tui

import (
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// If showing preview on narrow screen, go back to editor
			if m.previewShown && !m.isWideScreen() {
				m.previewShown = false
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "[":
			// Switch to Triggers tab
			if m.activeTab != tabTriggers {
				m.blurCurrentField()
				m.activeTab = tabTriggers
				m.focusIndex = 0
				m.focusCurrentField()
			}
			return m, nil

		case "]":
			// Switch to Actions tab
			if m.activeTab != tabActions {
				m.blurCurrentField()
				m.activeTab = tabActions
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
			m.focusIndex = 1 // First trigger's field selector
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
			m.focusIndex = 1 // First action's type selector
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

		case "+", "=":
			return m, m.handleAdd()

		case "-", "_":
			return m, m.handleRemove()

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
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput, cmd = m.triggers[itemIdx].valueInput.Update(msg)
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput, cmd = m.actions[itemIdx].valueInput.Update(msg)
		}
	}

	return m, cmd
}

// isInTextInput returns true if the current focus is on a text input field.
func (m *WorkflowRuleCreatorModel) isInTextInput() bool {
	section, _, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		return true
	case "trigger":
		return fieldIdx == 2 // value input
	case "action":
		return fieldIdx == 1 // value input
	}
	return false
}

// blurCurrentField removes focus from the currently focused field.
func (m *WorkflowRuleCreatorModel) blurCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Blur()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Blur()
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput.Blur()
		}
	}
}

// focusCurrentField sets focus on the currently focused field.
func (m *WorkflowRuleCreatorModel) focusCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Focus()
	case "trigger":
		if fieldIdx == 2 && itemIdx < len(m.triggers) {
			m.triggers[itemIdx].valueInput.Focus()
		}
	case "action":
		if fieldIdx == 1 && itemIdx < len(m.actions) {
			m.actions[itemIdx].valueInput.Focus()
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
		if itemIdx < len(m.actions) && fieldIdx == 0 {
			if m.actions[itemIdx].typeCursor > 0 {
				m.actions[itemIdx].typeCursor--
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
		if itemIdx < len(m.actions) && fieldIdx == 0 {
			if m.actions[itemIdx].typeCursor < len(m.actionTypes)-1 {
				m.actions[itemIdx].typeCursor++
			}
		}
	}
	return nil
}

// handleUp moves to the previous item (trigger or action).
func (m *WorkflowRuleCreatorModel) handleUp() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
		// Already at top, wrap to last item or logic
		if m.activeTab == tabTriggers {
			if len(m.triggers) > 1 {
				// Go to logic toggle
				m.focusIndex = 1 + len(m.triggers)*3
			} else {
				// Go to last trigger's first field
				m.focusIndex = 1
			}
		} else {
			// Go to last action's first field
			m.focusIndex = 1 + (len(m.actions)-1)*2
		}
	case "trigger":
		if itemIdx > 0 {
			// Go to previous trigger's first field
			m.focusIndex = 1 + (itemIdx-1)*3
		} else {
			// Go to name
			m.focusIndex = 0
		}
	case "logic":
		// Go to last trigger's first field
		m.focusIndex = 1 + (len(m.triggers)-1)*3
	case "action":
		if itemIdx > 0 {
			// Go to previous action's first field
			m.focusIndex = 1 + (itemIdx-1)*2
		} else {
			// Go to name
			m.focusIndex = 0
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// handleDown moves to the next item (trigger or action).
func (m *WorkflowRuleCreatorModel) handleDown() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
		// Go to first item's first field
		m.focusIndex = 1
	case "trigger":
		if itemIdx < len(m.triggers)-1 {
			// Go to next trigger's first field
			m.focusIndex = 1 + (itemIdx+1)*3
		} else if len(m.triggers) > 1 {
			// Go to logic toggle
			m.focusIndex = 1 + len(m.triggers)*3
		} else {
			// Wrap to name
			m.focusIndex = 0
		}
	case "logic":
		// Wrap to name
		m.focusIndex = 0
	case "action":
		if itemIdx < len(m.actions)-1 {
			// Go to next action's first field
			m.focusIndex = 1 + (itemIdx+1)*2
		} else {
			// Wrap to name
			m.focusIndex = 0
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// updateScrollForFocus adjusts scroll offset to keep focused item visible.
func (m *WorkflowRuleCreatorModel) updateScrollForFocus() {
	section, itemIdx, _ := m.getFocusSection()

	// Estimate line position of focused item
	// Name takes ~2 lines, section header ~2 lines
	// Each trigger takes ~4 lines, each action ~3 lines
	var focusLine int
	switch section {
	case "name":
		focusLine = 0
	case "trigger":
		focusLine = 4 + itemIdx*4
	case "logic":
		focusLine = 4 + len(m.triggers)*4
	case "action":
		focusLine = 4 + itemIdx*3
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

// handleAdd adds a new trigger or action based on current tab.
func (m *WorkflowRuleCreatorModel) handleAdd() tea.Cmd {
	factory := DefaultFormFieldFactory()

	if m.activeTab == tabTriggers {
		// Add a new trigger
		valueInput := factory.NewTextInput("Value")
		m.triggers = append(m.triggers, TriggerFormState{valueInput: valueInput})
	} else {
		// Add a new action
		valueInput := factory.NewTextInput("Value")
		m.actions = append(m.actions, ActionFormState{valueInput: valueInput})
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
