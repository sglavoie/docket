package tui

import tea "charm.land/bubbletea/v2"

// handleViewSupportMessages handles attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskAttachmentsMsg:
		if msg.Task == nil || msg.Project == nil {
			return true, nil
		}
		view := NewTaskAttachmentsModel(
			msg.Task,
			msg.Project,
			m.runtime,
			m.secondaryFeatures.Attachment,
		)
		return m.showViewInit(ViewTaskAttachments, view)

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		return true, nil
	}

	return false, nil
}
