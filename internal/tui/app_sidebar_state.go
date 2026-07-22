package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// setProjectSidebarEnabled commits the project's machine-local preference,
// then synchronizes every live peer model.
func (m *AppModel) setProjectSidebarEnabled(projectID string, enabled bool) {
	if projectID == "" {
		return
	}

	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.SetProjectSidebarEnabled(projectID, enabled)
	}); err != nil {
		logging.Warn("failed to save project sidebar state: %v", err)
	}
	m.syncProjectSidebarEnabled(projectID, enabled)
}

// determiningProjectSidebarEnabled resolves the project whose state controls
// which action-specific bulk sidebar command is available. A live primary
// project view is authoritative; global views fall back to remembered machine
// navigation when that project is active, then the first active project, then
// the default enabled state.
func (m *AppModel) determiningProjectSidebarEnabled() bool {
	if enabled, ok := m.liveProjectSidebarEnabled(); ok {
		return enabled
	}
	if m.runtime == nil {
		return true
	}
	projects := m.runtime.ActiveProjects()
	if len(projects) == 0 {
		return true
	}
	machine := m.runtime.Machine()
	for _, project := range projects {
		if project.ID == machine.CurrentProject {
			return machineProjectSidebarEnabled(machine, project.ID)
		}
	}
	return machineProjectSidebarEnabled(machine, projects[0].ID)
}

func (m *AppModel) liveProjectSidebarEnabled() (bool, bool) {
	view := m.currentView()
	if view == ViewCommandPalette && len(m.viewStack) > 1 {
		view = m.viewStack[len(m.viewStack)-2]
	}

	switch view { //nolint:exhaustive // Only project outliner presentations own sidebar state.
	case ViewOutliner:
		if m.views != nil {
			if outliner, ok := m.views.Get(ViewOutliner).(*OutlinerModel); ok && outliner.project != nil {
				return outliner.sidebar.IsEnabled(), true
			}
		}
		if m.outliner != nil && m.outliner.project != nil {
			return m.outliner.sidebar.IsEnabled(), true
		}
	case ViewTemporaryOutliner:
		if m.views != nil {
			if outliner, ok := m.views.Get(ViewTemporaryOutliner).(*OutlinerModel); ok && outliner.project != nil {
				return outliner.sidebar.IsEnabled(), true
			}
		}
	}
	return false, false
}

func machineProjectSidebarEnabled(machine *config.MachineState, projectID string) bool {
	if machine == nil {
		return true
	}
	return machine.GetProjectSidebarEnabled(projectID)
}

// setAllProjectSidebarsEnabled commits all active project preferences together,
// then synchronizes every matching live project view. Cross-project Outliner
// state is separate and intentionally untouched.
func (m *AppModel) setAllProjectSidebarsEnabled(enabled bool) {
	if m.runtime == nil {
		return
	}
	projects := m.runtime.ActiveProjects()
	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		for _, project := range projects {
			machine.SetProjectSidebarEnabled(project.ID, enabled)
		}
	}); err != nil {
		logging.Warn("failed to save project sidebar states: %v", err)
	}
	for _, project := range projects {
		m.syncProjectSidebarEnabled(project.ID, enabled)
	}
}

func (m *AppModel) syncProjectSidebarEnabled(projectID string, enabled bool) {
	setOutliner := func(outliner *OutlinerModel) {
		if outliner == nil || outliner.project == nil || outliner.project.ID != projectID {
			return
		}
		outliner.sidebar.SetEnabled(enabled)
		if !enabled {
			outliner.sidebar.Blur()
		}
	}

	// The typed pointer is the normal primary-view reference. Applying to it
	// also keeps isolated AppModel tests and models between registration
	// transitions consistent.
	setOutliner(m.outliner)
	setOutliner(m.pinnedPreview)

	if m.views == nil {
		return
	}
	for _, viewType := range []PresentationID{ViewOutliner, ViewTemporaryOutliner} {
		if outliner, ok := m.views.Get(viewType).(*OutlinerModel); ok {
			setOutliner(outliner)
		}
	}
}
