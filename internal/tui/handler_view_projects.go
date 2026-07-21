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
				m.showView(ViewTemporaryOutliner, fresh)
				return true, nil
			}
			if m.typedNavigation && m.taskWorkspaceView != nil {
				m.currentProject = msg.Project
				m.prepareProjectOutliner(msg.Project, msg.OpenAtTask)
				route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
					Scope: ProjectEntityScope(msg.Project.ID), Layout: WorkspaceOutline, Focus: WorkspaceActive,
				})
				if err != nil {
					return true, func() tea.Msg { return ErrorMsg{Err: err} }
				}
				cmd := m.activateTaskWorkspaceRoute(route)
				if msg.OpenAtTask != nil {
					m.taskWorkspace.State.SetCursor(TaskIdentity{ProjectID: msg.Project.ID, TaskID: msg.OpenAtTask.ID})
				}
				return true, cmd
			}

			replacing := m.isOnProjectView()

			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
			m.outliner = NewOutlinerModel(msg.Project, m.configManager, m.runtime)
			m.outliner.queueIDs = m.focusQueueIDs
			m.outliner.dailyPlanIDs = m.dailyPlanIDs
			m.restoreOutlinerToggleState()
			m.restoreOutlinerFilterState()
			// Persist the applied state so config stays in sync with the
			// in-memory filter state.
			m.saveOutlinerFilterState()

			if replacing {
				m.replaceView(ViewOutliner, m.outliner)
			} else {
				m.showView(ViewOutliner, m.outliner)
			}
		}
		return true, nil

	case CloseOutlinerMsg:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.popView()
		m.outliner = nil
		return true, nil

	case ShowGlobalOutlinerMsg:
		if m.typedNavigation && m.taskWorkspaceView != nil {
			presentationCmd := m.prepareCrossProjectOutliner()
			route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{
				Scope: AppEntityScope(), Layout: WorkspaceOutline, Focus: WorkspaceActive,
			})
			if err != nil {
				return true, func() tea.Msg { return ErrorMsg{Err: err} }
			}
			return true, tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)
		}
		// Save current view state so it persists if a different project's
		// view is later created from inside the cross-outliner.
		if m.outliner != nil {
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
		}
		if m.crossOutliner == nil {
			m.crossOutliner = NewOutlinerModel(nil, m.configManager, m.runtime)
			m.crossOutliner.queueIDs = m.focusQueueIDs
			m.crossOutliner.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewCrossProjectOutliner, m.crossOutliner)
			return true, m.crossOutliner.Init()
		}
		if m.currentView() == ViewCrossProjectOutliner {
			return true, m.crossOutliner.reloadCrossProjects()
		}
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return true, m.crossOutliner.reloadCrossProjects()

	case CloseGlobalOutlinerMsg:
		return true, m.popToAnchorView()

	case CloseCrossOutlinerMsg:
		return true, m.popToAnchorView()
	}

	return false, nil
}

// Renderer-scoped commands and scenario assertions inspect these model caches.
// They are not pushed or rendered; typed TaskWorkspaceView is the live surface.
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
