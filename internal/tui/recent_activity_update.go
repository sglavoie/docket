package tui

import tea "charm.land/bubbletea/v2"

func (m *RecentActivityModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadTasks, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m *RecentActivityModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.applyFilter()
		m.clampCursor()
	}); handled {
		return model, cmd
	}

	// Dispatcher handles all registered keys (navigation, filter prefix, actions).
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}

	switch msg.String() {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		if m.projectScope != nil {
			return m, func() tea.Msg {
				return CloseProjectRecentActivityMsg{}
			}
		}
		return m, func() tea.Msg {
			return CloseRecentActivityMsg{}
		}

	case "?":
		viewType := ViewRecentActivity
		viewName := "Recent Activity"
		if m.projectScope != nil {
			viewType = ViewProjectRecentActivity
			viewName = "Project Activity"
		}
		return m, helpCmd(viewType, viewName, m.GetHelpItems())
	}

	return m, nil
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *RecentActivityModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	// All filter categories enabled for recent activity
	hints := buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true)
	return &PrefixPopupInfo{
		Prefix: m.Dispatcher.PendingPrefix(),
		Hints:  hints,
	}
}
