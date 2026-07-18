package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// setProjectSidebarEnabled commits the sidebar preference shared by a
// project's Tree and Outliner views, then synchronizes every live peer model.
func (m *AppModel) setProjectSidebarEnabled(projectID string, enabled bool) {
	if projectID == "" {
		return
	}

	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		state, _ := machine.GetProjectViewState(projectID)
		state.SidebarEnabled = enabled
		machine.SetProjectViewState(projectID, state)
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

	switch view {
	case ViewProjectTree:
		if m.views != nil {
			if tree, ok := m.views.Get(ViewProjectTree).(*ProjectTreeModel); ok && tree.project != nil {
				return tree.sidebar.IsEnabled(), true
			}
		}
		if m.projectTree != nil && m.projectTree.project != nil {
			return m.projectTree.sidebar.IsEnabled(), true
		}
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
	if machine != nil {
		if state, ok := machine.GetProjectViewState(projectID); ok {
			return state.SidebarEnabled
		}
	}
	return true
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
			state, _ := machine.GetProjectViewState(project.ID)
			state.SidebarEnabled = enabled
			machine.SetProjectViewState(project.ID, state)
		}
	}); err != nil {
		logging.Warn("failed to save project sidebar states: %v", err)
	}
	for _, project := range projects {
		m.syncProjectSidebarEnabled(project.ID, enabled)
	}
}

func (m *AppModel) syncProjectSidebarEnabled(projectID string, enabled bool) {
	setTree := func(tree *ProjectTreeModel) {
		if tree == nil || tree.project == nil || tree.project.ID != projectID {
			return
		}
		tree.sidebar.SetEnabled(enabled)
		if !enabled {
			tree.sidebar.Blur()
		}
	}
	setOutliner := func(outliner *OutlinerModel) {
		if outliner == nil || outliner.project == nil || outliner.project.ID != projectID {
			return
		}
		outliner.sidebar.SetEnabled(enabled)
		if !enabled {
			outliner.sidebar.Blur()
		}
	}

	// The typed pointers are the normal primary-view references. Applying to
	// them also keeps isolated AppModel tests and models between registration
	// transitions consistent.
	setTree(m.projectTree)
	setOutliner(m.outliner)

	if m.views == nil {
		return
	}
	if tree, ok := m.views.Get(ViewProjectTree).(*ProjectTreeModel); ok {
		setTree(tree)
	}
	for _, viewType := range []ViewType{ViewOutliner, ViewTemporaryOutliner} {
		if outliner, ok := m.views.Get(viewType).(*OutlinerModel); ok {
			setOutliner(outliner)
		}
	}
}
