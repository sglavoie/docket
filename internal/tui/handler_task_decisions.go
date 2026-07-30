package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskDecisionMessages handles task decision messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskDecisionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case DecisionAddedMsg:
		return m.handleDecisionAdded(msg)

	case DecisionDeletedMsg:
		return m.handleDecisionDeleted(msg)

	case DecisionEditedMsg:
		return m.handleDecisionEdited(msg)

	}
	return false, nil
}

// handleDecisionAdded handles the DecisionAddedMsg.
func (m *AppModel) handleDecisionAdded(msg DecisionAddedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Decision added")
	viewCmd := m.delegateToCurrentView(msg)
	return true, tea.Batch(defaultStatusTimeout(), viewCmd, m.refreshAfterSubviewCommit())
}

// handleDecisionDeleted handles the DecisionDeletedMsg.
func (m *AppModel) handleDecisionDeleted(msg DecisionDeletedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Decision deleted")
	return true, tea.Batch(defaultStatusTimeout(), m.refreshAfterSubviewCommit())
}

// handleDecisionEdited handles the DecisionEditedMsg.
func (m *AppModel) handleDecisionEdited(msg DecisionEditedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Decision updated")
	return true, tea.Batch(defaultStatusTimeout(), m.refreshAfterSubviewCommit())
}
