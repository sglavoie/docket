package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// handleLeft moves cursor left within selectors.
func (m *WorkflowRuleCreatorModel) handleLeft() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	switch section {
	case "trigger":
		if itemIdx < len(m.triggers) {
			switch fieldIdx {
			case 0:
				if m.triggers[itemIdx].fieldCursor > 0 {
					m.triggers[itemIdx].fieldCursor--
				}
			case 1:
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
			case 0:
				if m.actions[itemIdx].typeCursor > 0 {
					m.actions[itemIdx].typeCursor--
				}
			case 2:
				if m.actions[itemIdx].condFieldCursor > 0 {
					m.actions[itemIdx].condFieldCursor--
				}
			case 3:
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
			case 0:
				if m.triggers[itemIdx].fieldCursor < len(m.triggerFields)-1 {
					m.triggers[itemIdx].fieldCursor++
				}
			case 1:
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
			case 0:
				if m.actions[itemIdx].typeCursor < len(m.actionTypes)-1 {
					m.actions[itemIdx].typeCursor++
				}
			case 2:
				if m.actions[itemIdx].condFieldCursor < len(m.triggerFields)-1 {
					m.actions[itemIdx].condFieldCursor++
				}
			case 3:
				if m.actions[itemIdx].condOpCursor < len(m.triggerOps)-1 {
					m.actions[itemIdx].condOpCursor++
				}
			}
		}
	}

	return nil
}

// handleUp moves to the previous item.
func (m *WorkflowRuleCreatorModel) handleUp() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
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
			m.focusIndex = 4
		}
	case "priority":
		m.focusIndex = 0
	case "trigger":
		if itemIdx > 0 {
			m.focusIndex = 2 + (itemIdx-1)*3
		} else {
			m.focusIndex = 1
		}
	case "logic":
		m.focusIndex = 2 + (len(m.triggers)-1)*3
	case "action":
		if itemIdx > 0 {
			m.focusIndex = 2 + m.actionFieldOffset(itemIdx-1)
		} else {
			m.focusIndex = 1
		}
	case "scope":
		if fieldIdx > 0 {
			m.focusIndex = fieldIdx + 1
		} else {
			m.focusIndex = 1
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}

// handleDown moves to the next item.
func (m *WorkflowRuleCreatorModel) handleDown() tea.Cmd {
	section, itemIdx, fieldIdx := m.getFocusSection()
	m.blurCurrentField()

	switch section {
	case "name":
		m.focusIndex = 1
	case "priority":
		m.focusIndex = 2
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
			m.focusIndex = fieldIdx + 3
		} else {
			m.focusIndex = 0
		}
	}

	m.focusCurrentField()
	m.updateScrollForFocus()
	return nil
}
