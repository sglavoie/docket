package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
)

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case projecthub.ShowProjectHubMsg:
		return true, m.activateManageHub(route.ManageParams{
			Module:         route.ManageProjects,
			ProjectHubMode: msg.Mode,
		})

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
			m.activateTaskWorkspaceRoute(route.OutlinerRootRoute()),
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
		// The one live Outliner is the one surface that ranks notes. Every other
		// construction site leaves the accessor nil, so the tier is simply absent
		// there — correct for a test model and for the Hub's task preview.
		m.crossOutliner.setNoteSearchIndex(m.noteSearchIndexRows)
		m.views.Register(presentation.ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	}
	return m.crossOutliner.reloadCrossProjects()
}
