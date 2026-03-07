package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles state transitions for the smart list builder.
func (m *SmartListBuilderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "[":
			if m.activeTab > slTabCriteria {
				m.blurCurrentField()
				m.activeTab--
				m.focusIndex = 0
				m.focusCurrentField()
			}
			return m, nil

		case "]":
			if m.activeTab < slTabOptions {
				m.blurCurrentField()
				m.activeTab++
				m.focusIndex = 0
				m.focusCurrentField()
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
			return m, m.saveSmartList()

		case "+", "=":
			if m.activeTab == slTabCriteria {
				m.addCriterion()
			}
			return m, nil

		case "-", "_":
			if m.activeTab == slTabCriteria {
				section, itemIdx, _ := m.getFocusSection()
				if section == "criterion" {
					m.removeCriterion(itemIdx)
				}
			}
			return m, nil

		case " ":
			section, _, _ := m.getFocusSection()
			switch section {
			case "logic":
				if m.logic == "and" {
					m.logic = "or"
				} else {
					m.logic = "and"
				}
			case "sort_dir":
				m.sortAscending = !m.sortAscending
			case "include_completed":
				m.includeCompleted = !m.includeCompleted
			}
			return m, nil

		case "left":
			return m, m.handleLeft()

		case "right":
			return m, m.handleRight()

		case "up":
			return m, m.handleUp()

		case "down":
			return m, m.handleDown()

		case "h":
			if !m.isInTextInput() {
				return m, m.handleLeft()
			}

		case "l":
			if !m.isInTextInput() {
				return m, m.handleRight()
			}

		case "k":
			if !m.isInTextInput() {
				return m, m.handleUp()
			}

		case "j":
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
	case "criterion":
		if fieldIdx == 2 && itemIdx < len(m.criteria) && !m.isEnumValue(itemIdx) {
			m.criteria[itemIdx].valueInput, cmd = m.criteria[itemIdx].valueInput.Update(msg)
		}
	}

	return m, cmd
}

// isInTextInput returns true if the current focus is on a text input field.
func (m *SmartListBuilderModel) isInTextInput() bool {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		return true
	case "criterion":
		return fieldIdx == 2 && !m.isEnumValue(itemIdx)
	}
	return false
}

// blurCurrentField removes focus from the currently focused field.
func (m *SmartListBuilderModel) blurCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Blur()
	case "criterion":
		if fieldIdx == 2 && itemIdx < len(m.criteria) {
			m.criteria[itemIdx].valueInput.Blur()
		}
	}
}

// focusCurrentField sets focus on the currently focused field.
func (m *SmartListBuilderModel) focusCurrentField() {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "name":
		m.nameInput.Focus()
	case "criterion":
		if fieldIdx == 2 && itemIdx < len(m.criteria) && !m.isEnumValue(itemIdx) {
			m.criteria[itemIdx].valueInput.Focus()
		}
	}
}

// handleLeft moves cursor left within selectors.
func (m *SmartListBuilderModel) handleLeft() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "criterion":
		if itemIdx < len(m.criteria) {
			switch fieldIdx {
			case 0: // field selector
				if m.criteria[itemIdx].fieldCursor > 0 {
					m.criteria[itemIdx].fieldCursor--
					// Reset op and value cursors when field changes
					m.criteria[itemIdx].opCursor = 0
					m.criteria[itemIdx].valueCursor = 0
				}
			case 1: // op selector
				if m.criteria[itemIdx].opCursor > 0 {
					m.criteria[itemIdx].opCursor--
				}
			case 2: // value selector (enum only)
				if m.isEnumValue(itemIdx) && m.criteria[itemIdx].valueCursor > 0 {
					m.criteria[itemIdx].valueCursor--
				}
			}
		}
	case "logic":
		if m.logic == "or" {
			m.logic = "and"
		}
	case "sort_field":
		if m.sortField > 0 {
			m.sortField--
		}
	case "sort_dir":
		m.sortAscending = !m.sortAscending
	case "include_completed":
		m.includeCompleted = !m.includeCompleted
	}
	return nil
}

// handleRight moves cursor right within selectors.
func (m *SmartListBuilderModel) handleRight() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "criterion":
		if itemIdx < len(m.criteria) {
			switch fieldIdx {
			case 0: // field selector
				if m.criteria[itemIdx].fieldCursor < len(smartListFields)-1 {
					m.criteria[itemIdx].fieldCursor++
					m.criteria[itemIdx].opCursor = 0
					m.criteria[itemIdx].valueCursor = 0
				}
			case 1: // op selector
				field := m.currentField(itemIdx)
				if m.criteria[itemIdx].opCursor < len(field.Operations)-1 {
					m.criteria[itemIdx].opCursor++
				}
			case 2: // value selector (enum only)
				if m.isEnumValue(itemIdx) {
					field := m.currentField(itemIdx)
					if m.criteria[itemIdx].valueCursor < len(field.EnumValues)-1 {
						m.criteria[itemIdx].valueCursor++
					}
				}
			}
		}
	case "logic":
		if m.logic == "and" {
			m.logic = "or"
		}
	case "sort_field":
		if m.sortField < len(slSortFieldNames)-1 {
			m.sortField++
		}
	case "sort_dir":
		m.sortAscending = !m.sortAscending
	case "include_completed":
		m.includeCompleted = !m.includeCompleted
	}
	return nil
}

// handleUp moves to the previous focusable item.
func (m *SmartListBuilderModel) handleUp() tea.Cmd {
	m.blurCurrentField()
	total := m.focusFieldCount()
	m.focusIndex = (m.focusIndex + total - 1) % total
	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// handleDown moves to the next focusable item.
func (m *SmartListBuilderModel) handleDown() tea.Cmd {
	m.blurCurrentField()
	total := m.focusFieldCount()
	m.focusIndex = (m.focusIndex + 1) % total
	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// updateScrollForFocus adjusts scroll offset to keep focused item visible.
func (m *SmartListBuilderModel) updateScrollForFocus() {
	section, itemIdx, _ := m.getFocusSection()

	var focusLine int
	switch section {
	case "name":
		focusLine = 0
	case "criterion":
		focusLine = 4 + itemIdx*4
	case "logic":
		focusLine = 4 + len(m.criteria)*4
	case "sort_field":
		focusLine = 2
	case "sort_dir":
		focusLine = 5
	case "include_completed":
		focusLine = 8
	}

	visibleHeight := m.height - 7
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

// saveSmartList validates and saves the smart list.
func (m *SmartListBuilderModel) saveSmartList() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("smart list name is required")}
		}
		if len(m.criteria) == 0 {
			return ErrorMsg{Err: fmt.Errorf("at least one criterion is required")}
		}

		sl := m.buildSmartList()
		isNew := m.editingID == ""

		if isNew {
			if err := m.config.AddSmartList(sl); err != nil {
				return ErrorMsg{Err: err}
			}
		} else {
			m.config.UpdateSmartList(sl)
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}

		return SmartListSavedMsg{SmartList: sl, IsNew: isNew}
	}
}
