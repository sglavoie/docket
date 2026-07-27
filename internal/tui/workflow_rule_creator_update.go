package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// Update handles state transitions for the workflow rule creator.
func (m *WorkflowRuleCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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
	case tea.KeyPressMsg:
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

		case "shift+left":
			// Switch to previous tab
			if m.activeTab > tabTriggers {
				m.blurCurrentField()
				m.activeTab--
				m.focusIndex = 0
				m.focusCurrentField()
			}
			return m, nil

		case "shift+right":
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

		case "space":
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
			m.scopeStatesInput, cmd = m.scopeStatesInput.Update(msg)
		}
	}

	return m, cmd
}
