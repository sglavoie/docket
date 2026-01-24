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
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "ctrl+f":
			// Open field picker for current or first trigger
			section, itemIdx, _ := m.getFocusSection()
			targetIdx := 0
			if section == "trigger" {
				targetIdx = itemIdx
			}
			if targetIdx < len(m.triggers) {
				currentField := m.triggerFields[m.triggers[targetIdx].fieldCursor]
				m.fieldPicker = NewTriggerFieldPickerModel(targetIdx, currentField)
				m.fieldPicker.SetSize(m.width, m.height)
				m.fieldPickerTarget = targetIdx
				m.showingFieldPicker = true
			}
			return m, nil

		case "ctrl+t":
			// Open action type picker for current or first action
			section, itemIdx, _ := m.getFocusSection()
			targetIdx := 0
			if section == "action" {
				targetIdx = itemIdx
			}
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

// handleAdd adds a new trigger or action based on current focus.
func (m *WorkflowRuleCreatorModel) handleAdd() tea.Cmd {
	section, _, _ := m.getFocusSection()
	factory := DefaultFormFieldFactory()

	switch section {
	case "trigger", "name":
		// Add a new trigger
		valueInput := factory.NewTextInput("Value")
		m.triggers = append(m.triggers, TriggerFormState{valueInput: valueInput})
	case "action", "logic":
		// Add a new action
		valueInput := factory.NewTextInput("Value")
		m.actions = append(m.actions, ActionFormState{valueInput: valueInput})
	}
	return nil
}

// handleRemove removes the current trigger or action.
func (m *WorkflowRuleCreatorModel) handleRemove() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()

	switch section {
	case "trigger":
		if len(m.triggers) > 1 && itemIdx < len(m.triggers) {
			m.triggers = append(m.triggers[:itemIdx], m.triggers[itemIdx+1:]...)
			// Adjust focus if needed
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
	case "action":
		if len(m.actions) > 1 && itemIdx < len(m.actions) {
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
