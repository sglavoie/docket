package tui

import tea "charm.land/bubbletea/v2"

// handleViewSupportMessages handles blockers and attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowBlockersMsg:
		if msg.Project == nil {
			return true, m.activateManageProjectPicker(ManageBlockers)
		}
		return true, m.activateManageHub(ManageParams{
			Module: ManageBlockers,
			Scope:  ProjectEntityScope(msg.Project.ID),
		})

	case ShowGlobalBlockersMsg:
		return true, m.activateManageHub(ManageParams{Module: ManageBlockers, Scope: AppEntityScope()})

	case BlockerGoToProjectMsg:
		m.popView()
		if msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.pushView(ViewOutliner)
		}
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		return true, m.activateTaskDetail(msg.Task, msg.Project, TaskAttachments)

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		return true, nil
	}

	return false, nil
}
