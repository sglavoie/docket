package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, &m.filter.ScoreMin, func() { m.rebuildRows() })
		return m, cmd
	}

	// Handle search mode input first
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.rebuildRows()
		DetectSearchAutocomplete(m.SearchAC(), m.SearchInputValue(), m.searchTagCache(), m.searchSectionCache())
	}); handled {
		return model, cmd
	}

	// Handle quick edit mode input first
	if m.quickEdit.IsActive() {
		return m.handleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.confirm.IsActive() {
		return m.handleConfirmDialog(key)
	}

	// Sidebar-focused key handling (before prefix/action keys)
	if m.sidebar.IsFocused() {
		wasEnabled := m.sidebar.IsEnabled()
		_, cmd := m.sidebar.HandleKeys(m, msg)
		projectID := ""
		if m.project != nil {
			projectID = m.project.ID
		}
		stateCmd := projectSidebarStateChangedCmd(projectID, wasEnabled, m.sidebar.IsEnabled())
		// When sidebar is focused, consume all keys to prevent tree navigation
		if stateCmd == nil {
			return m, cmd
		}
		return m, tea.Batch(cmd, stateCmd)
	}

	// Dispatcher handles prefix keys and registered key actions.
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}

	// Multi-select mode key handling (when active, intercept selection keys not handled by dispatcher)
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		}
	}

	// Cross-project mode: intercept creation keys before they reach the shared handler.
	if model, cmd, handled := m.crossInterceptCreationKeys(key); handled {
		return model, cmd
	}

	// Cross-project mode: C opens the context picker, including from the empty state.
	if model, cmd, handled := m.crossHandleContextPickerKey(key); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleNavigationKey(key); handled {
		return model, cmd
	}

	// onHeader is true when the cursor rests on any non-task row (section header,
	// separator, or project sub-header).
	onHeader := len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) &&
		!m.rows[m.cursor].isTaskRow()

	if model, cmd, handled := m.handleTaskMutationKey(key, onHeader); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleTaskViewKey(key, onHeader); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleUtilityKey(key); handled {
		return model, cmd
	}

	return m, nil
}
