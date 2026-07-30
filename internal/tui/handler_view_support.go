package tui

import tea "charm.land/bubbletea/v2"

// handleViewSupportMessages handles attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskAttachmentsMsg:
		return true, m.activateTaskDetail(msg.Task, msg.Project, TaskAttachments)

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		return true, nil
	}

	return false, nil
}
