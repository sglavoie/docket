package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"
)

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
	}

	return m, nil
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *RecentActivityModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	hints := PrefixHints{}
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.runtime.Machine(), true, true, false))
	if _, ok := hints[prefix]; !ok {
		fallback := make([]PrefixHint, 0, len(m.Dispatcher.PrefixHints()))
		for _, h := range m.Dispatcher.PrefixHints() {
			fallback = append(fallback, PrefixHint{Key: h.Key, Description: h.Description})
		}
		hints[prefix] = fallback
	}
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  hints,
	}
}
