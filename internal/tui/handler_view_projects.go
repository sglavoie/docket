package tui

import tea "charm.land/bubbletea/v2"

// handleProjectViewMessages handles project tree and outliner view transitions.
func (m *AppModel) handleProjectViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowOutlinerMsg:
		if msg.Project != nil {
			if msg.Temporary {
				fresh := NewOutlinerModel(msg.Project, m.config, m.configManager)
				fresh.queueIDs = m.focusQueueIDs
				fresh.OpenAtTask = msg.OpenAtTask
				m.showView(ViewTemporaryOutliner, fresh)
				return true, nil
			}

			replacing := m.isOnProjectView()
			var shared *sharedViewState
			if replacing && m.projectTree != nil {
				shared = m.extractSharedStateFromTree()
			}

			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
			m.outliner = NewOutlinerModel(msg.Project, m.config, m.configManager)
			m.outliner.queueIDs = m.focusQueueIDs
			m.restoreOutlinerToggleState()
			if shared != nil {
				m.applySharedStateToOutliner(shared)
			} else {
				m.restoreOutlinerFilterState()
			}

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
		if m.crossOutliner == nil {
			m.crossOutliner = NewOutlinerModel(nil, m.config, m.configManager)
			m.crossOutliner.queueIDs = m.focusQueueIDs
			m.showView(ViewCrossProjectOutliner, m.crossOutliner)
			return true, m.crossOutliner.Init()
		}
		if m.currentView() == ViewCrossProjectOutliner {
			return true, m.crossOutliner.reloadCrossProjects()
		}
		m.showView(ViewCrossProjectOutliner, m.crossOutliner)
		return true, m.crossOutliner.reloadCrossProjects()

	case CloseGlobalOutlinerMsg:
		m.saveCrossOutlinerState()
		m.popToAnchorView()
		return true, nil

	case CloseCrossOutlinerMsg:
		m.saveCrossOutlinerState()
		m.popToAnchorView()
		return true, nil

	case ShowProjectTreeMsg:
		if msg.Project != nil {
			replacing := m.isOnProjectView()
			if replacing {
				var shared *sharedViewState
				if m.outliner != nil {
					shared = m.extractSharedStateFromOutliner()
				}

				m.saveOutlinerToggleState()
				m.saveOutlinerFilterState()
				m.refreshProjectTree()
				if shared != nil {
					m.applySharedStateToTree(shared)
				}
				m.replaceView(ViewProjectTree, m.projectTree)
			} else {
				if m.projectTree == nil || m.currentProject == nil || msg.Project.ID != m.currentProject.ID {
					m.switchToProject(msg.Project, true)
				}
				m.showView(ViewProjectTree, m.projectTree)
			}
		}
		return true, nil
	}

	return false, nil
}
