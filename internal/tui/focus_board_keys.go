package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
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

	// Reorder today items
	switch key {
	case "J":
		return m.handleDailyPlanReorder(1)
	case "K":
		return m.handleDailyPlanReorder(-1)
	}

	// Multi-select key handling
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case "space":
			m.toggleTaskSelection()
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAll()
			return m, nil
		}
		switch key {
		case ",", "!", "d", "s", "m", ".":
			m.pendingPrefix = key
			return m, nil
		}
	}

	// Activate prefix keys: F always, ,!g gated on item, mc gated on item
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmctb.", m.getRowAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}

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
	case "shift+left", "shift+right":
		if handleTabCycleKey(key, &m.tabs, func() {
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
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
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

	if m.multiSelectMode && m.SelectedCount() > 0 {
		if model, cmd, handled := dispatchBulkPrefix(prefix, key, m, m.getSelectedBulkItems()); handled {
			m.ExitMultiSelect()
			return model, cmd
		}
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

// Multi-select helpers.

func (m *FocusBoardModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, "Multi-select: space=toggle, a=all, A=deselect")
}

func (m *FocusBoardModel) toggleTaskSelection() {
	row := m.getRowAtCursor()
	if row == nil || !row.isTask() {
		return
	}
	m.ToggleAndUpdateStatus(row.Task.ID, &m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusBoardModel) selectAllVisible() {
	for _, row := range m.filtered {
		if row.isTask() {
			m.selectedTasks[row.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

func (m *FocusBoardModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, "Multi-select: space=toggle", "%d selected")
}

// getSelectedBulkItems returns selected tasks as bulkTaskItem slice.
func (m *FocusBoardModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, row := range m.filtered {
		if row.isTask() && m.selectedTasks[row.Task.ID] {
			items = append(items, bulkTaskItem{Task: row.Task, Project: row.Project})
		}
	}
	return items
}

func (m *FocusBoardModel) HandledKeys() []string {
	return concatKeys(
		[]string{"j", "k", "J", "K", "G", "ctrl+u", "ctrl+d"}, // FocusBoard registry has j/k and ctrl+u/d but not ↑/↓
		statusPrefixKeys(),
		priorityPrefixKeys(),
		dueDatePrefixKeys(),
		snoozePrefixKeys(),
		linkedTaskPrefixKeys(),
		stagePrefixKeys(),
		blockerPrefixKeySet(),
		commonActionKeys(),
		[]string{
			"g", "gp",
			"tab", "shift+left", "shift+right",
			"zp",
			"enter", "a", "x", "X",
			"d", "p", "E", "t", "m",
			"F", "Fx", "/",
			"1", "2", "3", "4", "5", "0",
			"v", "w", "u", "esc",
		},
	)
}
