package tui

import tea "charm.land/bubbletea/v2"

// handleViewSelectionMessages handles transient pickers, previews, and status editors.
func (m *AppModel) handleViewSelectionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		if msg.CrossProject {
			m.taskJumper = NewCrossProjectTaskJumperModel(msg.Projects)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		return true, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil && m.crossOutliner != nil {
			m.crossOutliner.NavigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project, m.runtime))
		}
		return true, nil

	case ShowStatusIconPickerMsg:
		if msg.Project != nil {
			m.showView(ViewStatusIconPicker, NewStatusIconPickerModel(msg.Project))
		}
		return true, nil

	case StatusIconSelectedMsg:
		popCmd := m.popView()
		if msg.Project == nil {
			return true, popCmd
		}
		if msg.Canceled {
			return true, popCmd
		}

		m.showView(ViewStatusInput, newStatusInputModelWithIcon(msg.Project, m.runtime, msg.Key))
		return true, nil

	case StatusUpdatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.refreshScopeHubIfActive()
		m.setSuccessStatus("Status updated")
		return true, tea.Batch(popCmd, m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())
	}

	return false, nil
}
