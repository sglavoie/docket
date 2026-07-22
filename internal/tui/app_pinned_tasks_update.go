package tui

import (
	"fmt"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// handlePinnedPopupKey handles key events when the pinned tasks popup is open.
func (m *AppModel) handlePinnedPopupKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if m.pinnedPopupFiltering {
		return m.handlePinnedPopupFilterKey(msg)
	}
	return m.handlePinnedPopupNormalKey(msg)
}

// handlePinnedPopupNormalKey handles keys in normal (non-filtering) mode.
func (m *AppModel) handlePinnedPopupNormalKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.pinnedPopupCursor < m.pinnedPopupFilteredLen()-1 {
			m.pinnedPopupCursor++
		}
		return m, nil
	case "k", "up":
		if m.pinnedPopupCursor > 0 {
			m.pinnedPopupCursor--
		}
		return m, nil
	case "enter":
		return m.pinnedPopupSelectFiltered(m.pinnedPopupCursor)
	case "shift+enter":
		return m.pinnedPopupPeekFiltered(m.pinnedPopupCursor)
	case "p":
		return m.unpinFromPopup()
	case "J":
		return m.reorderPinnedTask(1)
	case "K":
		return m.reorderPinnedTask(-1)
	case "esc":
		if m.pinnedPopupFilter != "" {
			m.pinnedPopupFilter = ""
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
			return m, nil
		}
		m.closePinnedTasksOverlay()
		m.invalidatePinnedPreview()
		return m, nil
	case "/":
		m.pinnedPopupFiltering = true
		m.pinnedPopupFilter = ""
		m.pinnedPopupCursor = 0
		m.rebuildPinnedPopupFilter()
		return m, nil
	default:
		return m, nil
	}
}

// handlePinnedPopupFilterKey handles keys in filter (type-to-search) mode.
func (m *AppModel) handlePinnedPopupFilterKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up":
		if m.pinnedPopupCursor > 0 {
			m.pinnedPopupCursor--
		}
		return m, nil
	case "down":
		if m.pinnedPopupCursor < m.pinnedPopupFilteredLen()-1 {
			m.pinnedPopupCursor++
		}
		return m, nil
	case "enter":
		return m.pinnedPopupSelectFiltered(m.pinnedPopupCursor)
	case "shift+enter":
		return m.pinnedPopupPeekFiltered(m.pinnedPopupCursor)
	case "esc":
		m.pinnedPopupFilter = ""
		m.pinnedPopupFiltering = false
		m.pinnedPopupCursor = 0
		m.rebuildPinnedPopupFilter()
		return m, nil
	case "backspace":
		if len(m.pinnedPopupFilter) > 0 {
			m.pinnedPopupFilter = m.pinnedPopupFilter[:len(m.pinnedPopupFilter)-1]
			if m.pinnedPopupFilter == "" {
				m.pinnedPopupFiltering = false
			}
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
		}
		return m, nil
	default:
		runes := []rune(msg.Text)
		if len(runes) == 1 && unicode.IsPrint(runes[0]) {
			m.pinnedPopupFilter += string(runes[0])
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
			return m, nil
		}
		return m, nil
	}
}

// pinnedPopupPeekFiltered opens Peek above Pinned without disturbing Pinned's
// order, cursor, or active filter.
func (m *AppModel) pinnedPopupPeekFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.pinnedPopupFilteredLen() {
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[filteredIdx]
	entry := m.pinnedPopupItems[realIdx]
	m.activatePeek(entry.Task, entry.Project)
	return m, nil
}

// pinnedPopupSelectFiltered selects the entry at the given filtered index.
func (m *AppModel) pinnedPopupSelectFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.pinnedPopupFilteredLen() {
		m.closePinnedTasksOverlay()
		m.invalidatePinnedPreview()
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[filteredIdx]
	return m.pinnedPopupSelect(realIdx)
}

// pinnedPopupSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) pinnedPopupSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.pinnedPopupItems) {
		m.closePinnedTasksOverlay()
		m.invalidatePinnedPreview()
		return m, nil
	}
	entry := m.pinnedPopupItems[idx]
	m.closePinnedTasksOverlay()
	m.invalidatePinnedPreview()
	return m, m.navigateToTaskInProject(entry.Task, entry.Project)
}

// reorderPinnedTask moves the current pinned item by delta (+1 = down, -1 = up)
// by swapping the two tasks' PinnedAt ordering through the application layer (one
// atomic, ledger-undoable transaction across the affected projects), then
// reloads them and rebuilds the popup.
func (m *AppModel) reorderPinnedTask(delta int) (*AppModel, tea.Cmd) {
	target := m.pinnedPopupCursor + delta
	if target < 0 || target >= m.pinnedPopupFilteredLen() {
		return m, nil
	}

	realA := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	realB := m.pinnedPopupFiltered[target]
	a := m.pinnedPopupItems[realA]
	b := m.pinnedPopupItems[realB]

	res, err := m.appService.ReorderPinnedTasksByLocator(
		taskIdentityFor(a.Task, a.Project).Locator(), taskIdentityFor(b.Task, b.Project).Locator(),
	)
	if err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to reorder pinned tasks: %v", err))
		return m, nil
	}
	for _, projectID := range res.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}

	m.pinnedPopupItems = m.collectPinnedTasks()
	m.rebuildPinnedPopupFilter()
	m.pinnedPopupCursor = target

	return m, nil
}

// unpinFromPopup unpins the currently selected task and removes it from the popup.
func (m *AppModel) unpinFromPopup() (*AppModel, tea.Cmd) {
	if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= m.pinnedPopupFilteredLen() {
		return m, nil
	}

	realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	entry := m.pinnedPopupItems[realIdx]
	res, err := m.appService.SetPinnedByLocator(taskIdentityFor(entry.Task, entry.Project).Locator(), false)
	if err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to unpin task: %v", err))
		return m, nil
	}
	m.reloadProjectByID(res.ProjectID)

	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.closePinnedTasksOverlay()
	} else {
		m.rebuildPinnedPopupFilter()
		if m.pinnedPopupFilteredLen() == 0 {
			m.pinnedPopupFilter = ""
			m.pinnedPopupFiltering = false
			m.rebuildPinnedPopupFilter()
		}
		m.pinnedPopupClampCursor()
	}

	return m, nil
}
