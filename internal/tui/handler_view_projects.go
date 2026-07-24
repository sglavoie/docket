package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowOutlinerMsg:
		if msg.Project != nil {
			return true, m.enterProjectScopeOutliner(msg.Project, msg.OpenAtTask, msg.Options)
		}
		return true, nil

	case CloseOutlinerMsg:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.popView()
		m.outliner = nil
		return true, nil

	case ShowGlobalOutlinerMsg:
		// Save current view state so it persists if a different project's
		// view is later created from inside the cross-outliner.
		if m.outliner != nil {
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
		}
		presentationCmd := m.prepareCrossProjectOutliner()
		m.crossOutliner.ApplyOpenOptions(msg.Options)
		if m.typedNavigation {
			route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
			if err != nil {
				return true, func() tea.Msg { return ErrorMsg{Err: err} }
			}
			return true, tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)
		}
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return true, presentationCmd

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

	if m.typedNavigation {
		// A project-navigation request abandons any transient overlay and child
		// route and lands on the sole app-scoped Outliner root.
		m.taskDetail = nil
		m.taskComposer = nil
		if cmd := m.activateTaskWorkspaceRoute(outlinerRootRoute()); cmd != nil {
			return cmd
		}
	} else {
		m.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}
	}

	cross := m.crossOutliner
	if cross == nil {
		return presentationCmd
	}
	cross.pushProjectScope(project.ID)
	if options.SetMode || options.SetFocus || options.SetSort || options.SetLens {
		cross.ApplyOpenOptions(options)
	} else {
		// Explicit project navigation presents the project's full task tree in
		// All rather than inheriting another mode, so project navigation exposes
		// the project's full workflow plus its scoped Today section.
		cross.selectOutlinerMode(ModeAll)
	}
	if openAt != nil {
		cross.NavigateToTaskInProject(openAt, project)
	}
	m.setAccentOnView(cross)

	// Project work is served by the scoped cross Outliner; discard any transient
	// single-project instance switchToProject may have created so one
	// authoritative Outliner owns all project-scoped work.
	if m.outliner != nil {
		m.outliner = nil
		m.views.Remove(ViewOutliner)
	}
	return presentationCmd
}

func (m *AppModel) prepareCrossProjectOutliner() tea.Cmd {
	if m.crossOutliner == nil {
		m.crossOutliner = NewOutlinerModel(nil, m.configManager, m.runtime)
		m.crossOutliner.queueIDs = m.focusQueueIDs
		m.crossOutliner.dailyPlanIDs = m.dailyPlanIDs
		m.crossOutliner.SetSize(m.width, m.height-2)
		m.wireSecondaryCoordinators(m.crossOutliner)
		m.views.Register(ViewCrossProjectOutliner, m.crossOutliner)
		return m.crossOutliner.Init()
	}
	return m.crossOutliner.reloadCrossProjects()
}
