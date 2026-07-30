package tui

import (
	tea "charm.land/bubbletea/v2"
)

// ApplyProjectScopeMsg applies a Context Hub or project-header selection to the
// sole Outliner scope stack.
type ApplyProjectScopeMsg struct {
	ProjectID string
	Clear     bool
}

func (m *AppModel) applyProjectScope(msg ApplyProjectScopeMsg) (bool, tea.Cmd) {
	if m.baseProjectView() != ViewCrossProjectOutliner || m.crossOutliner == nil {
		m.setInfoStatus("Project scope is available from the global Outliner")
		return true, nil
	}
	returnToOutliner := m.currentView() == ViewContextHub
	if msg.Clear {
		m.crossOutliner.clearProjectScope()
		if returnToOutliner {
			return true, m.returnToOutlinerAfterContextSelection()
		}
		return true, nil
	}
	if !m.crossOutliner.projectIsLoaded(msg.ProjectID) {
		m.setInfoStatus("That project isn't loaded — restore it before scoping")
		return true, nil
	}
	m.crossOutliner.pushProjectScope(msg.ProjectID)
	if returnToOutliner {
		return true, m.returnToOutlinerAfterContextSelection()
	}
	return true, nil
}
