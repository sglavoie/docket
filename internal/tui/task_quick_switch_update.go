package tui

import (
	"fmt"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// handleQuickSwitchKey handles key events when the quick-switch popup is open.
func (m *AppModel) handleQuickSwitchKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if m.quickSwitchFiltering {
		return m.handleQuickSwitchFilterKey(msg)
	}
	return m.handleQuickSwitchNormalKey(msg)
}

// handleQuickSwitchNormalKey handles keys in normal (non-filtering) mode.
func (m *AppModel) handleQuickSwitchNormalKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		if m.quickSwitchCursor < m.quickSwitchFilteredLen()-1 {
			m.quickSwitchCursor++
		}
		return m, nil
	case "k", "up":
		if m.quickSwitchCursor > 0 {
			m.quickSwitchCursor--
		}
		return m, nil
	case "enter":
		return m.quickSwitchSelectFiltered(m.quickSwitchCursor)
	case "shift+enter":
		return m.quickSwitchPeekFiltered(m.quickSwitchCursor)
	case "esc":
		if m.quickSwitchFilter != "" {
			m.quickSwitchFilter = ""
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
			return m, nil
		}
		m.closeQuickSwitchOverlay()
		return m, nil
	case "p":
		m.quickSwitchTogglePin()
		return m, nil
	case "/":
		m.quickSwitchFiltering = true
		m.quickSwitchFilter = ""
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		return m, nil
	default:
		return m, nil
	}
}

// handleQuickSwitchFilterKey handles keys in filter (type-to-search) mode.
func (m *AppModel) handleQuickSwitchFilterKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up":
		if m.quickSwitchCursor > 0 {
			m.quickSwitchCursor--
		}
		return m, nil
	case "down":
		if m.quickSwitchCursor < m.quickSwitchFilteredLen()-1 {
			m.quickSwitchCursor++
		}
		return m, nil
	case "enter":
		return m.quickSwitchSelectFiltered(m.quickSwitchCursor)
	case "shift+enter":
		return m.quickSwitchPeekFiltered(m.quickSwitchCursor)
	case "esc":
		m.quickSwitchFilter = ""
		m.quickSwitchFiltering = false
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		return m, nil
	case "backspace":
		if len(m.quickSwitchFilter) > 0 {
			m.quickSwitchFilter = m.quickSwitchFilter[:len(m.quickSwitchFilter)-1]
			if m.quickSwitchFilter == "" {
				m.quickSwitchFiltering = false
			}
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
		}
		return m, nil
	default:
		runes := []rune(msg.Text)
		if len(runes) == 1 && unicode.IsPrint(runes[0]) {
			m.quickSwitchFilter += string(runes[0])
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
			return m, nil
		}
		return m, nil
	}
}

// quickSwitchPeekFiltered opens Peek above Recent while keeping Recent's
// cursor and filter state parked underneath it.
func (m *AppModel) quickSwitchPeekFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.quickSwitchFilteredLen() {
		return m, nil
	}
	realIdx := m.quickSwitchFiltered[filteredIdx]
	entry := m.recentTaskVisits[realIdx]
	m.activatePeek(entry.Task, entry.Project)
	return m, nil
}

// quickSwitchTogglePin toggles the pin state of the entry at the cursor.
func (m *AppModel) quickSwitchTogglePin() {
	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return
	}

	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	entry := &m.recentTaskVisits[realIdx]

	if !entry.Pinned {
		pinnedCount := 0
		for _, existingEntry := range m.recentTaskVisits {
			if existingEntry.Pinned {
				pinnedCount++
			}
		}
		if pinnedCount >= maxPinnedTaskVisits {
			m.setInfoStatus(fmt.Sprintf("Maximum %d pinned tasks reached", maxPinnedTaskVisits))
			return
		}
	}

	entry.Pinned = !entry.Pinned

	identity := entry.Identity()
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
	for i, filteredIdx := range m.quickSwitchFiltered {
		if m.recentTaskVisits[filteredIdx].Identity() == identity {
			m.quickSwitchCursor = i
			break
		}
	}

	m.persistRecentTaskVisits()
}
