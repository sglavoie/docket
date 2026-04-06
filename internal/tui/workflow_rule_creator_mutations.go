package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// handleAdd adds a new trigger or action based on the current tab.
func (m *WorkflowRuleCreatorModel) handleAdd() tea.Cmd {
	factory := DefaultFormFieldFactory()
	if m.activeTab == tabTriggers {
		valueInput := factory.NewTextInput("Value")
		m.triggers = append(m.triggers, TriggerFormState{valueInput: valueInput, groupIndex: -1})
		return nil
	}

	valueInput := factory.NewTextInput("Value")
	condValueInput := factory.NewTextInput("Condition value")
	m.actions = append(m.actions, ActionFormState{valueInput: valueInput, condValueInput: condValueInput})
	return nil
}

// handleRemove removes the focused trigger or action while preserving minimum counts.
func (m *WorkflowRuleCreatorModel) handleRemove() tea.Cmd {
	section, itemIdx, _ := m.getFocusSection()
	if m.activeTab == tabTriggers {
		if section == "trigger" && len(m.triggers) > 1 && itemIdx < len(m.triggers) {
			m.triggers = append(m.triggers[:itemIdx], m.triggers[itemIdx+1:]...)
			total := m.focusFieldCount()
			if m.focusIndex >= total {
				m.focusIndex = total - 1
			}
		}
		return nil
	}

	if section == "action" && len(m.actions) > 1 && itemIdx < len(m.actions) {
		m.actions = append(m.actions[:itemIdx], m.actions[itemIdx+1:]...)
		total := m.focusFieldCount()
		if m.focusIndex >= total {
			m.focusIndex = total - 1
		}
	}
	return nil
}

// toggleTriggerGroup toggles a trigger in or out of a sub-group.
func (m *WorkflowRuleCreatorModel) toggleTriggerGroup(triggerIdx int) {
	trigger := &m.triggers[triggerIdx]
	if trigger.groupIndex >= 0 {
		oldGroup := trigger.groupIndex
		trigger.groupIndex = -1
		m.cleanupEmptyGroups(oldGroup)
		return
	}

	if len(m.triggerGroups) == 0 {
		m.triggerGroups = append(m.triggerGroups, TriggerGroupFormState{logic: workflow.LogicAnd})
	}
	trigger.groupIndex = 0
}

// cleanupEmptyGroups removes a group if it no longer has enough members.
func (m *WorkflowRuleCreatorModel) cleanupEmptyGroups(groupIdx int) {
	count := 0
	for _, trigger := range m.triggers {
		if trigger.groupIndex == groupIdx {
			count++
		}
	}
	if count >= 2 {
		return
	}

	for i := range m.triggers {
		if m.triggers[i].groupIndex == groupIdx {
			m.triggers[i].groupIndex = -1
		}
	}

	m.triggerGroups = append(m.triggerGroups[:groupIdx], m.triggerGroups[groupIdx+1:]...)
	for i := range m.triggers {
		if m.triggers[i].groupIndex > groupIdx {
			m.triggers[i].groupIndex--
		}
	}
}
