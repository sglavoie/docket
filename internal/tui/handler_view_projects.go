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
				fresh.dailyPlanIDs = m.dailyPlanIDs
				fresh.OpenAtTask = msg.OpenAtTask
				m.restoreOutlinerFilterStateTo(fresh)
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
			m.outliner.dailyPlanIDs = m.dailyPlanIDs
			m.restoreOutlinerToggleState()
			if shared != nil {
				m.applySharedStateToOutliner(shared)
			} else {
				m.restoreOutlinerFilterState()
			}
			// Persist the applied state so config stays in sync with the
			// in-memory filter state (guards against refreshProjectTree
			// restoring stale config values).
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
		// Save current view state so it persists if a different project's
		// view is later created from inside the cross-outliner.
		m.saveProjectTreeState()
		if m.outliner != nil {
			m.saveOutlinerToggleState()
			m.saveOutlinerFilterState()
		}
		if m.crossOutliner == nil {
			m.crossOutliner = NewOutlinerModel(nil, m.config, m.configManager)
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
		m.saveCrossOutlinerState()
		return true, m.popToAnchorView()

	case CloseCrossOutlinerMsg:
		m.saveCrossOutlinerState()
		return true, m.popToAnchorView()

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
					// Persist the applied shared state so config stays in
					// sync (guards against SaveCompleteMsg/RefreshMsg calling
					// refreshProjectTree and restoring stale tree filters).
					m.saveProjectTreeState()
				}
				m.replaceView(ViewProjectTree, m.projectTree)
			} else {
				if m.projectTree == nil || m.currentProject == nil || msg.Project.ID != m.currentProject.ID {
					m.switchToProject(msg.Project, true)
				} else {
					m.currentProject = msg.Project
					m.refreshProjectTree()
				}
				m.showView(ViewProjectTree, m.projectTree)
			}
		}
		return true, nil
	}

	return false, nil
}
