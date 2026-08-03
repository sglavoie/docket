package tui

import tea "charm.land/bubbletea/v2"

// handleViewActionMessages handles task action views such as priority, capture, and attachments.
func (m *AppModel) handleViewActionMessages(msg tea.Msg) (bool, tea.Cmd) {
	if scope, ok := msg.(ApplyProjectScopeMsg); ok {
		return m.applyProjectScope(scope)
	}
	return m.handleCaptureAndBulkMessages(msg)
}
