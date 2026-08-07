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
	return true, tea.Batch(m.setRecoveryFeedback("Checklist item added", msg.TransactionID, false), m.refreshAfterSubviewCommit())
}

// handleChecklistItemDeleted handles the ChecklistItemDeletedMsg.
func (m *AppModel) handleChecklistItemDeleted(msg ChecklistItemDeletedMsg) (bool, tea.Cmd) {
	return true, tea.Batch(m.setRecoveryFeedback("Checklist item deleted", msg.TransactionID, false), m.refreshAfterSubviewCommit())
}

// handleChecklistItemEdited handles the ChecklistItemEditedMsg.
func (m *AppModel) handleChecklistItemEdited(msg ChecklistItemEditedMsg) (bool, tea.Cmd) {
	return true, tea.Batch(m.setRecoveryFeedback("Checklist item updated", msg.TransactionID, false), m.refreshAfterSubviewCommit())
}

// handleChecklistItemToggled handles the ChecklistItemToggledMsg.
func (m *AppModel) handleChecklistItemToggled(msg ChecklistItemToggledMsg) (bool, tea.Cmd) {
	return true, tea.Batch(m.setRecoveryFeedback("Checklist item toggled", msg.TransactionID, false), m.refreshAfterSubviewCommit())
}
