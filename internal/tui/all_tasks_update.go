package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *AllTasksViewModel) Init() tea.Cmd {
	return nil
}

func (m *AllTasksViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadAllTasks, m.clampCursor, func(s string) { m.setStatus(s) }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case AllTasksRefreshMsg:
		m.loadAllTasks()
		m.clampCursor()
		return m, nil

	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *AllTasksViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.overflowPickerOpen {
		return m.handleOverflowPickerKey(msg)
	}
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	return m, nil
}

// handleOverflowPickerKey handles keys while the overflow tab picker is open.
func (m *AllTasksViewModel) handleOverflowPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	overflowEntries := m.allTabEntries[overflowTabCap-1:]
	count := len(overflowEntries)
	switch msg.String() {
	case "esc":
		m.overflowPickerOpen = false
	case "j", "down":
		if m.overflowPickerCursor < count-1 {
			m.overflowPickerCursor++
		}
	case "k", "up":
		if m.overflowPickerCursor > 0 {
			m.overflowPickerCursor--
		}
	case "enter":
		m.activeTabIdx = (overflowTabCap - 1) + m.overflowPickerCursor
		m.tabs.Active = len(m.tabEntries) - 1 // keep "More…" visually active
		m.overflowPickerOpen = false
		if m.activeTabIncludesCompleted() != m.loadedWithCompleted {
			m.loadAllTasks()
		} else {
			m.applyFiltersAndGroup()
		}
		m.clampCursor()
	}
	return m, nil
}

// escSteps defines the Esc dismissal cascade: multi-select → filters.
func (m *AllTasksViewModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, multi-select, confirm, or overflow picker are active.
func (m *AllTasksViewModel) ShouldCaptureEsc() bool {
	if m.overflowPickerOpen {
		return true
	}
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}
