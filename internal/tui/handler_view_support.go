package tui

import "charm.land/bubbletea/v2"

// handleViewSupportMessages handles blockers and attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowBlockersMsg:
		if msg.Project != nil {
			editor := m.config.Editor
			m.showView(ViewBlockers, NewBlockersModelWithEditor(msg.Project, editor))
		}
		return true, nil

	case ShowGlobalBlockersMsg:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.config))
		return true, nil

	case BlockerGoToProjectMsg:
		m.popView()
		if msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(msg.Task, msg.Project, m.config))
		}
		return true, nil

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case AttachmentReorderedMsg:
		return true, nil
	}

	return false, nil
}
