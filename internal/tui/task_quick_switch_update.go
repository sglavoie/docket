package tui

import (
	"fmt"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// handleQuickSwitchKey handles key events when the quick-switch popup is open.
func (m *AppModel) handleQuickSwitchKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if m.quickSwitchPendingPrefix != "" {
		return m.handleQuickSwitchPrefixKey(msg)
	}
	if m.quickSwitchFiltering {
		return m.handleQuickSwitchFilterKey(msg)
	}
	return m.handleQuickSwitchNormalKey(msg)
}

// handleQuickSwitchPrefixKey dispatches the second key of a `,` or `!` prefix.
func (m *AppModel) handleQuickSwitchPrefixKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if isModifierOnly(msg) {
		return m, nil
	}
	prefix := m.quickSwitchPendingPrefix
	m.quickSwitchPendingPrefix = ""
	key := msg.String()

	if key == "esc" {
		return m, nil
	}

	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return m, nil
	}
	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	entry := m.recentTaskVisits[realIdx]

	if prefix == "," {
		status, ok := StatusForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyStatusChange(simplePersistWithUndo(m.persister, m.config, StatusVerb(status)+" Task"), entry.Project, entry.Task.ID, status)
		m.setSuccessStatus(result.StatusMessage)
	}

	if prefix == "!" {
		priority, ok := PriorityForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyPriorityChange(simplePersistWithUndo(m.persister, m.config, "Set Priority"), entry.Project, entry.Task.ID, priority)
		m.setSuccessStatus(result.StatusMessage)
	}

	m.refreshRecentTaskVisits()
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
	m.quickSwitchClampCursor()

	return m, nil
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
	case "esc":
		if m.quickSwitchFilter != "" {
			m.quickSwitchFilter = ""
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
			return m, nil
		}
		m.showQuickSwitch = false
		return m, nil
	case "|":
		m.showQuickSwitch = false
		return m, nil
	case "p":
		m.quickSwitchTogglePin()
		return m, nil
	case ",":
		m.quickSwitchPendingPrefix = ","
		return m, nil
	case "!":
		m.quickSwitchPendingPrefix = "!"
		return m, nil
	case "c":
		m.quickSwitchShowCompleted = !m.quickSwitchShowCompleted
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		m.quickSwitchClampCursor()
		return m, nil
	case "x":
		m.quickSwitchRemoveEntry()
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0]-'0') - 1
		return m.quickSwitchSelectFiltered(idx)
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
		m.quickSwitchFiltering = false
		return m, nil
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

	taskID := entry.Task.ID
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
	for i, filteredIdx := range m.quickSwitchFiltered {
		if m.recentTaskVisits[filteredIdx].Task.ID == taskID {
			m.quickSwitchCursor = i
			break
		}
	}

	m.persistRecentTaskVisits()
}

// quickSwitchRemoveEntry removes the entry at the cursor from the list.
func (m *AppModel) quickSwitchRemoveEntry() {
	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return
	}

	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	m.recentTaskVisits = append(m.recentTaskVisits[:realIdx], m.recentTaskVisits[realIdx+1:]...)

	if len(m.recentTaskVisits) == 0 {
		m.showQuickSwitch = false
		return
	}

	m.rebuildQuickSwitchFilter()
	if m.quickSwitchFilteredLen() == 0 {
		m.quickSwitchFilter = ""
		m.quickSwitchFiltering = false
		m.rebuildQuickSwitchFilter()
	}
	m.quickSwitchClampCursor()

	m.persistRecentTaskVisits()
}
