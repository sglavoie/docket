package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) handleQuickSwitchToggle() tea.Cmd {
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		return func() tea.Msg { return TutorDismissedMsg{} }
	}
	if m.showQuickSwitch {
		m.showQuickSwitch = false
		return nil
	}
	m.openQuickSwitch()
	if len(m.recentTaskVisits) == 0 {
		m.showQuickSwitch = false
		m.setInfoStatus("No recent tasks for quick-switch")
		return defaultStatusTimeout()
	}
	return nil
}

func (m *AppModel) handlePinnedPopupToggle() tea.Cmd {
	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.setInfoStatus("No pinned tasks — press p on a task to pin it")
		return defaultStatusTimeout()
	}
	m.showPinnedPopup = !m.showPinnedPopup
	m.pinnedPopupCursor = 0
	m.pinnedPopupFilter = ""
	m.pinnedPopupFiltering = false
	m.rebuildPinnedPopupFilter()
	return nil
}

func (m *AppModel) handleUndoToggle() tea.Cmd {
	switch m.currentView() { //nolint:exhaustive
	case ViewFocusView, ViewFocusBoard:
		if cmd := m.undoDailyPlanToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
		if cmd := m.undoQueueToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
	}
	return nil
}

func (m *AppModel) handleRedoToggle() tea.Cmd {
	switch m.currentView() { //nolint:exhaustive
	case ViewFocusView, ViewFocusBoard:
		if cmd := m.redoDailyPlanToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
		if cmd := m.redoQueueToggle(); cmd != nil {
			return m.batchWithFocusRefresh(cmd)
		}
	}
	return nil
}
