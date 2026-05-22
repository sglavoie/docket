package tui

import tea "charm.land/bubbletea/v2"

// handleViewActionMessages handles task action views such as priority, capture, blockers, and attachments.
func (m *AppModel) handleViewActionMessages(msg tea.Msg) (bool, tea.Cmd) {
	if handled, cmd := m.handleGlobalPickerMessages(msg); handled {
		return handled, cmd
	}
	return m.handleCaptureAndBulkMessages(msg)
}
