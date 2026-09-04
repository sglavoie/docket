package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// ApplyProjectScopeMsg applies a Project Hub or project-header selection to the
// sole Outliner scope stack.
type ApplyProjectScopeMsg struct {
	ProjectID string
	Clear     bool
}

func (m *AppModel) applyProjectScope(msg ApplyProjectScopeMsg) (bool, tea.Cmd) {
	if m.baseProjectView() != presentation.ViewCrossProjectOutliner || m.crossOutliner == nil {
		m.setInfoStatus("Project scope is available from the global Outliner")
		return true, nil
	}
	returnToOutliner := m.currentView() == presentation.ViewProjectHub
	if msg.Clear {
		// A lens applied under this scope recorded project facets it could not act
		// on, because the scope had already fixed the population. Leaving the scope
		// is when they decide it, so the exit rides that load instead of dropping
		// the scope onto the population it replaced.
		activation, deferred := m.crossOutliner.BeginLensScopeExit()
		if !deferred {
			m.crossOutliner.ClearProjectScope()
		}
		if returnToOutliner {
			return true, tea.Batch(activation, m.returnToOutlinerAfterProjectScope())
		}
		return true, activation
	}
	if !m.crossOutliner.ProjectIsLoaded(msg.ProjectID) {
		// A parked project keeps its tasks on disk but contributes nothing to the
		// base workspace, so opening it is an explicit lazy load rather than an
		// error. The scope is pushed by the load result, not here.
		if cmd, started := m.startParkedProjectFocus(msg.ProjectID); started {
			return true, cmd
		}
		m.setInfoStatus("That project isn't loaded — restore it before scoping")
		return true, nil
	}
	m.crossOutliner.PushProjectScope(msg.ProjectID)
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
	return m.crossOutliner.BeginParkedProjectFocus(projectID), true
}

// finishParkedProjectFocus closes the surface that requested a parked focus once
// the scope is actually in place. A load that failed, was superseded, or was
// abandoned leaves the requesting surface open, which is where its error belongs.
func (m *AppModel) finishParkedProjectFocus(focusProjectID string) tea.Cmd {
	if focusProjectID == "" || m.crossOutliner == nil {
		return nil
	}
	if m.crossOutliner.Scope.ProjectID != focusProjectID {
		return nil
	}
	if m.currentView() != presentation.ViewProjectHub {
		return nil
	}
	return m.returnToOutlinerAfterProjectScope()
}

// returnToOutlinerAfterProjectScope dismisses every primary child above the sole
// Outliner root without resetting the freshly applied project scope. Cancel uses
// the ordinary one-pop path instead so it can restore the prior view.
func (m *AppModel) returnToOutlinerAfterProjectScope() tea.Cmd {
	m.viewNav.navigation.ResetRoot()
	if view := m.viewNav.views.Get(m.currentView()); view != nil {
		m.setAccentOnView(view)
	}
	return func() tea.Msg { return status.ViewDidAppearMsg{} }
}
