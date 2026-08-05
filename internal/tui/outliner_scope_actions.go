package tui

import (
	tea "charm.land/bubbletea/v2"
)

// ApplyProjectScopeMsg applies a Project Hub or project-header selection to the
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
	returnToOutliner := m.currentView() == ViewProjectHub
	if msg.Clear {
		m.crossOutliner.clearProjectScope()
		if returnToOutliner {
			return true, m.returnToOutlinerAfterProjectScope()
		}
		return true, nil
	}
	if !m.crossOutliner.projectIsLoaded(msg.ProjectID) {
		// A parked project keeps its tasks on disk but contributes nothing to the
		// base workspace, so opening it is an explicit lazy load rather than an
		// error. The scope is pushed by the load result, not here.
		if cmd, started := m.startParkedProjectFocus(msg.ProjectID); started {
			return true, cmd
		}
		m.setInfoStatus("That project isn't loaded — restore it before scoping")
		return true, nil
	}
	m.crossOutliner.pushProjectScope(msg.ProjectID)
	if returnToOutliner {
		return true, m.returnToOutlinerAfterProjectScope()
	}
	return true, nil
}

// startParkedProjectFocus begins the lazy load that admits one parked project
// into the snapshot. The requesting surface stays open while the load runs, so a
// failure can report against the row the user chose instead of stranding them in
// an Outliner that never gained the project.
func (m *AppModel) startParkedProjectFocus(projectID string) (tea.Cmd, bool) {
	if m.runtime == nil || projectID == "" {
		return nil, false
	}
	project := m.runtime.AvailableProject(projectID)
	if project == nil {
		return nil, false
	}
	if m.runtime.TaskWorkspaceProject(projectID) != nil {
		// In the base workspace but absent from the snapshot: a failed or stale
		// load, not a parked project. Leave the existing guidance in place.
		return nil, false
	}
	return m.crossOutliner.beginParkedProjectFocus(projectID), true
}

// finishParkedProjectFocus closes the surface that requested a parked focus once
// the scope is actually in place. A load that failed, was superseded, or was
// abandoned leaves the requesting surface open, which is where its error belongs.
func (m *AppModel) finishParkedProjectFocus(focusProjectID string) tea.Cmd {
	if focusProjectID == "" || m.crossOutliner == nil {
		return nil
	}
	if m.crossOutliner.scopedProjectID != focusProjectID {
		return nil
	}
	if m.currentView() != ViewProjectHub {
		return nil
	}
	return m.returnToOutlinerAfterProjectScope()
}

// returnToOutlinerAfterProjectScope dismisses every primary child above the sole
// Outliner root without resetting the freshly applied project scope. Cancel uses
// the ordinary one-pop path instead so it can restore the prior view.
func (m *AppModel) returnToOutlinerAfterProjectScope() tea.Cmd {
	m.navigation.ResetRoot()
	if view := m.views.Get(m.currentView()); view != nil {
		m.setAccentOnView(view)
	}
	return func() tea.Msg { return ViewDidAppearMsg{} }
}
