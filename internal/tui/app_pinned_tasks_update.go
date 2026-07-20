package tui

import (
	"fmt"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// handlePinnedPopupKey handles key events when the pinned tasks popup is open.
func (m *AppModel) handlePinnedPopupKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	if m.pinnedPendingPrefix != "" && isModifierOnly(msg) {
		return m, nil
	}
	if m.pinnedPendingPrefix != "" {
		return m.handlePinnedPopupPrefixKey(msg)
	}
	if m.pinnedPopupFiltering {
		return m.handlePinnedPopupFilterKey(msg)
	}
	return m.handlePinnedPopupNormalKey(msg)
}

// handlePinnedPopupPrefixKey dispatches the second key of a `,` or `!` prefix.
func (m *AppModel) handlePinnedPopupPrefixKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	prefix := m.pinnedPendingPrefix
	m.pinnedPendingPrefix = ""
	key := msg.String()

	if key == "esc" {
		return m, nil
	}

	if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= m.pinnedPopupFilteredLen() {
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	entry := m.pinnedPopupItems[realIdx]

	if prefix == "!" {
		priority, ok := PriorityForPrefixKey(key)
		if !ok {
			return m, nil
		}
		res, err := m.appService.SetPriority(entry.Task.ID, priority)
		if err != nil {
			m.setInfoStatus(fmt.Sprintf("Failed to set priority: %v", err))
			return m, nil
		}
		m.reloadProjectByID(res.ProjectID)
		m.setSuccessStatus(fmt.Sprintf("Priority set to %d", priority))
	}

	if prefix == "," {
		state, ok := StateForPrefixKey(key)
		if !ok {
			return m, nil
		}
		res, err := m.appService.Transition(app.StateRequest{TaskID: entry.Task.ID, Target: state})
		if err != nil {
			m.setInfoStatus(fmt.Sprintf("Failed to change status: %v", err))
			return m, nil
		}
		m.reloadProjectByID(res.ProjectID)
		m.setSuccessStatus(fmt.Sprintf("Task #%d -> %s", res.ShortID, state))
	}

	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.showPinnedPopup = false
	} else {
		m.rebuildPinnedPopupFilter()
		m.pinnedPopupClampCursor()
	}

	return m, nil
}

// handlePinnedPopupNormalKey handles keys in normal (non-filtering) mode.
func (m *AppModel) handlePinnedPopupNormalKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	switch msg.String() {
	case ",":
		m.pinnedPendingPrefix = ","
		return m, nil
	case "!":
		m.pinnedPendingPrefix = "!"
		return m, nil
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
	case "u":
		return m.unpinFromPopup()
	case "r":
		return m.repinFromPopup()
	case "J":
		return m.reorderPinnedTask(1)
	case "K":
		return m.reorderPinnedTask(-1)
	case "ctrl+j":
		if m.pinnedPreview != nil && m.pinnedPreview.cursor < len(m.pinnedPreview.rows)-1 {
			m.pinnedPreview.cursor++
		}
		return m, nil
	case "ctrl+k":
		if m.pinnedPreview != nil && m.pinnedPreview.cursor > 0 {
			m.pinnedPreview.cursor--
		}
		return m, nil
	case "esc":
		if m.pinnedPopupFilter != "" {
			m.pinnedPopupFilter = ""
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
			return m, nil
		}
		m.showPinnedPopup = false
		m.pinnedPendingPrefix = ""
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	case "#":
		m.showPinnedPopup = false
		m.pinnedPendingPrefix = ""
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		return m.pinnedPopupSelectFiltered(idx)
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
		m.pinnedPopupFiltering = false
		return m, nil
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

// pinnedPopupSelectFiltered selects the entry at the given filtered index.
func (m *AppModel) pinnedPopupSelectFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.pinnedPopupFilteredLen() {
		m.showPinnedPopup = false
		m.invalidatePinnedPreview()
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[filteredIdx]
	return m.pinnedPopupSelect(realIdx)
}

// pinnedPopupSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) pinnedPopupSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.pinnedPopupItems) {
		m.showPinnedPopup = false
		m.invalidatePinnedPreview()
		return m, nil
	}
	entry := m.pinnedPopupItems[idx]
	m.showPinnedPopup = false
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

	res, err := m.appService.ReorderPinnedTasks(a.Task.ID, b.Task.ID)
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
	res, err := m.appService.SetPinned(entry.Task.ID, false)
	if err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to unpin task: %v", err))
		return m, nil
	}
	saved := entry
	m.pinnedLastUnpinned = &saved
	m.reloadProjectByID(res.ProjectID)

	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.showPinnedPopup = false
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

// repinFromPopup re-pins the last unpinned task (undo for unpin).
func (m *AppModel) repinFromPopup() (*AppModel, tea.Cmd) {
	if m.pinnedLastUnpinned == nil {
		return m, nil
	}

	entry := m.pinnedLastUnpinned
	res, err := m.appService.SetPinned(entry.Task.ID, true)
	if err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to repin task: %v", err))
		return m, nil
	}
	m.reloadProjectByID(res.ProjectID)

	m.pinnedLastUnpinned = nil
	m.pinnedPopupItems = m.collectPinnedTasks()
	m.rebuildPinnedPopupFilter()
	m.pinnedPopupClampCursor()

	return m, nil
}
