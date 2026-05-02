package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskChecklistMessages handles task checklist messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskChecklistMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ChecklistItemAddedMsg:
		return m.handleChecklistItemAdded(msg)

	case ChecklistItemDeletedMsg:
		return m.handleChecklistItemDeleted(msg)

	case ChecklistItemEditedMsg:
		return m.handleChecklistItemEdited(msg)

	case ChecklistItemToggledMsg:
		return m.handleChecklistItemToggled(msg)
	}
	return false, nil
}

// handleChecklistItemAdded handles the ChecklistItemAddedMsg.
func (m *AppModel) handleChecklistItemAdded(msg ChecklistItemAddedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Checklist item added")
	return true, defaultStatusTimeout()
}

// handleChecklistItemDeleted handles the ChecklistItemDeletedMsg.
func (m *AppModel) handleChecklistItemDeleted(msg ChecklistItemDeletedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Checklist item deleted")
	return true, defaultStatusTimeout()
}

// handleChecklistItemEdited handles the ChecklistItemEditedMsg.
func (m *AppModel) handleChecklistItemEdited(msg ChecklistItemEditedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Checklist item updated")
	return true, defaultStatusTimeout()
}

// handleChecklistItemToggled handles the ChecklistItemToggledMsg.
func (m *AppModel) handleChecklistItemToggled(msg ChecklistItemToggledMsg) (bool, tea.Cmd) {
	return true, nil
}
