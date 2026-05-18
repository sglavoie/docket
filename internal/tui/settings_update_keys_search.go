package tui

import tea "charm.land/bubbletea/v2"

// handleSearchKey consumes all keys when the settings search overlay is active.
func (m *SettingsModel) handleSearchKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.searchActive = false
		m.searchInput.Blur()
		return true, nil
	case "enter":
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			result := m.searchResults[m.searchCursor]
			m.blurCurrentField()
			m.focusIndex = result.FocusIndex
			m.tabs.Active = result.Tab
			m.subTab = result.SubTab
			m.syncVisualFromOld()
			m.focusCurrentField()
			m.scrollOffset = 0
		}
		m.searchActive = false
		m.searchInput.Blur()
		return true, nil
	case "j", "down", "ctrl+j":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
		return true, nil
	case "k", "up", "ctrl+k":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return true, nil
	default:
		old := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != old {
			m.searchResults = searchSettingsFields(m.searchInput.Value())
			m.searchCursor = 0
		}
		return true, cmd
	}
}
