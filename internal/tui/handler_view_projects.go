package tui

import tea "charm.land/bubbletea/v2"

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg.(type) {
	case ShowProjectHubMsg:
		return true, m.activateManageHub(ManageParams{Module: ManageProjects})

	case launchOutlinerMsg:
		presentationCmd := m.prepareCrossProjectOutliner()
		m.clearTransientNavigationState()
		m.taskComposer = nil
		m.currentProject = nil
		m.projectStack = nil
		m.navigation.ResetRoot()
		m.crossOutliner.resetToGlobalAll()
		m.setAccentOnView(m.crossOutliner)
		// The reset drops any parked focus after presentationCmd captured it, so
		// supersede that load with one that requests the base workspace alone.
		return true, tea.Batch(
			m.activateTaskWorkspaceRoute(outlinerRootRoute()),
			presentationCmd,
			m.crossOutliner.takeParkedFocusReload(),
		)

	}

	return false, nil
}

func (m *AppModel) prepareCrossProjectOutliner() tea.Cmd {
	if m.crossOutliner == nil {
		m.crossOutliner = NewOutlinerModel(nil, m.configManager, m.runtime)
		m.crossOutliner.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(m.crossOutliner)
		m.views.Register(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	}
	return m.crossOutliner.reloadCrossProjects()
}
