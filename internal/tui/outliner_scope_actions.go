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
	returnToOutliner := m.currentView() == ViewContextHub || m.currentView() == ViewProjectHub
	if msg.Clear {
		m.crossOutliner.clearProjectScope()
		if returnToOutliner {
			return true, m.returnToOutlinerAfterContextSelection()
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
		return true, m.returnToOutlinerAfterContextSelection()
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
	switch m.currentView() {
	case ViewProjectHub, ViewContextHub:
		return m.returnToOutlinerAfterContextSelection()
	default:
		return nil
	}
}
