package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirmation dialog before other key processing
	if m.confirm.IsActive() {
		return m.handleConfirmKey(key)
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	// Activate prefix keys: F always, ,!g gated on item, mc gated on item
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmc.", m.getRowAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	switch key {
	case "esc", "q":
		if m.HasActiveFilters() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
			return m, nil
		}
		return m, func() tea.Msg { return CloseFocusBoardMsg{} }

	// Search
	case "/":
		m.searchInput.SetValue(m.searchQuery)
		return m, m.ActivateSearch()

	// Tab navigation
	case "tab":
		m.tabs.Cycle(1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "shift+tab":
		m.tabs.Cycle(-1)
		m.applyFilter()
		m.clampCursor()
		return m, nil
	case "[", "h", "]", "l":
		if handleTabCycleKey(key, &m.tabs, true, func() {
			m.applyFilter()
			m.clampCursor()
		}) {
			return m, nil
		}

	// Quick-add action
	case "a":
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	// Quick-edit title
	case "E":
		if row := m.getRowAtCursor(); row != nil && row.isAction() {
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}

	// Mark done
	case "x":
		return m.markDone()

	// Dismiss (cancel)
	case "X":
		return m.dismiss()

	// Set priority
	case "p":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project}
			}
		}

	// Open URL / navigate to task
	case "enter":
		return m.handleEnter()

	// Open URL directly
	case "o":
		if row := m.getRowAtCursor(); row != nil && row.Task.URL != "" {
			return m, openBrowserCmd(row.Task.URL)
		}

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sort, func() {
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Sort reset")
		}, func(field SortField) {
			m.applyFilter()
			m.clampCursor()
			direction := "↑"
			if !m.sort.Ascending {
				direction = "↓"
			}
			m.setStatus(fmt.Sprintf("Sort: %s %s", field, direction))
		})
		return m, nil

	// Assign to project (move task to another project)
	case "P":
		if row := m.getRowAtCursor(); row != nil && row.Project != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: row.Task, SourceProject: row.Project}
			}
		}
		return m, nil

	// Score editor
	case "S":
		if row := m.getRowAtCursor(); row != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: row.Task, Project: row.Project}
			}
		}
		return m, nil

	}

	// Common task actions (t, Z, D, e, r, y/Y/ctrl+y, ?)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *FocusBoardModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		if pfx == "z" && k == "p" {
			m.previewMode = !m.previewMode
			return m, nil, true
		}
		return nil, nil, false
	}
	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	return m, nil
}

// handleConfirmKey handles key input while the confirmation dialog is active.
func (m *FocusBoardModel) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return m.deleteItem()
		}
		return m, nil
	}
	return m, nil
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewFocusBoard)
}
