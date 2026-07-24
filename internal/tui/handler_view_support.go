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
		// Return to the app-scoped Outliner root and push the selected project's
		// scope frame on the single authoritative cross Outliner rather than
		// reviving a separate project Outliner root.
		if msg.Project != nil {
			return true, m.enterProjectScopeOutliner(msg.Project, nil, OutlinerOpenOptions{})
		}
		return true, m.popToAnchorView()

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		return true, nil

	case ShowTaskAttachmentsMsg:
		return true, m.activateTaskDetail(msg.Task, msg.Project, TaskAttachments)

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		return true, nil
	}

	return false, nil
}
