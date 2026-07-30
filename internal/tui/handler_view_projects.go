package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case launchOutlinerMsg:
		presentationCmd := m.prepareCrossProjectOutliner()
		m.clearTransientNavigationState()
		m.taskDetail = nil
		m.taskComposer = nil
		m.projectStack = nil
		m.navigation.ResetRoot()
		m.crossOutliner.resetToGlobalAll()
		m.setAccentOnView(m.crossOutliner)
		return true, tea.Batch(m.activateTaskWorkspaceRoute(outlinerRootRoute()), presentationCmd)

	case ShowOutlinerMsg:
		if msg.Project != nil {
			return true, m.enterProjectScopeOutliner(msg.Project, msg.OpenAtTask, msg.Options)
		}
		return true, nil

	case CloseOutlinerMsg:
		m.saveOutlinerFilterState()
		m.popView()
		return true, nil

	case ShowGlobalOutlinerMsg:
		presentationCmd := m.prepareCrossProjectOutliner()
		m.crossOutliner.ApplyOpenOptions(msg.Options)
		route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
		if err != nil {
			return true, func() tea.Msg { return ErrorMsg{Err: err} }
		}
		return true, tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)

	case CloseGlobalOutlinerMsg:
		return true, m.popToAnchorView()

	case CloseCrossOutlinerMsg:
		return true, m.popToAnchorView()
	}

	return false, nil
}

// enterProjectScopeOutliner represents project selection as a scope frame on the
// single authoritative (cross-project) Outliner rather than constructing a
// separate project-scoped root. It returns to the Outliner root — dropping any
// child route and canceling overlays — pushes the project's scope frame, applies
// the requested open options, and reveals openAt when provided. The root route
// stays app-scoped; project identity lives only in the Outliner's scope stack, so
// Esc restores the previous scope and Ctrl+Esc restores clean global All.
func (m *AppModel) enterProjectScopeOutliner(project *models.Project, openAt *models.Task, options OutlinerOpenOptions) tea.Cmd {
	if project == nil {
		return nil
	}
	m.currentProject = project

	var presentationCmd tea.Cmd
	if m.crossOutliner == nil {
		presentationCmd = m.prepareCrossProjectOutliner()
	}

	// A project-navigation request abandons any transient overlay and child route
	// and lands on the sole app-scoped Outliner root.
	m.taskDetail = nil
	m.taskComposer = nil
	if cmd := m.activateTaskWorkspaceRoute(outlinerRootRoute()); cmd != nil {
		return cmd
	}

	cross := m.crossOutliner
	if cross == nil {
		return presentationCmd
	}
	cross.pushProjectScope(project.ID)
	if options.SetRouteState || options.SetMode || options.SetSort {
		cross.ApplyOpenOptions(options)
	}
	if openAt != nil {
		cross.NavigateToTaskInProject(openAt, project)
	}
	m.setAccentOnView(cross)

	return presentationCmd
}

func (m *AppModel) prepareCrossProjectOutliner() tea.Cmd {
	if m.crossOutliner == nil {
		m.crossOutliner = NewOutlinerModel(nil, m.configManager, m.runtime)
		m.crossOutliner.todayIDs = m.todayIDs
		m.crossOutliner.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(m.crossOutliner)
		m.views.Register(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	}
	return m.crossOutliner.reloadCrossProjects()
}
