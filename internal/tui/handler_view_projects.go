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
			if msg.Temporary {
				fresh := NewOutlinerModel(msg.Project, m.configManager, m.runtime)
				fresh.queueIDs = m.focusQueueIDs
				fresh.dailyPlanIDs = m.dailyPlanIDs
				fresh.OpenAtTask = msg.OpenAtTask
				m.restoreOutlinerFilterStateTo(fresh)
				fresh.ApplyOpenOptions(msg.Options)
				m.showView(ViewTemporaryOutliner, fresh)
				return true, nil
			}
			m.currentProject = msg.Project
			m.prepareProjectOutliner(msg.Project, msg.OpenAtTask)
			m.outliner.ApplyOpenOptions(msg.Options)
			if m.typedNavigation {
				route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: ProjectEntityScope(msg.Project.ID)})
				if err != nil {
					return true, func() tea.Msg { return ErrorMsg{Err: err} }
				}
				return true, m.activateTaskWorkspaceRoute(route)
			}
			m.showView(ViewOutliner, m.outliner)
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

// The mature Outliner models are the canonical project and cross-project Task
// Workspace renderers.
func (m *AppModel) prepareProjectOutliner(project *models.Project, openAt *models.Task) {
	if project == nil {
		return
	}
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
	m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
	m.outliner.queueIDs = m.focusQueueIDs
	m.outliner.dailyPlanIDs = m.dailyPlanIDs
	m.outliner.OpenAtTask = openAt
	m.restoreOutlinerToggleState()
	m.restoreOutlinerFilterState()
	m.outliner.SetSize(m.width, m.height-2)
	m.wireSecondaryCoordinators(m.outliner)
	m.views.Register(ViewOutliner, m.outliner)
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
