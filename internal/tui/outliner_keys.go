package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg processes key events in the outliner view.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, m.config, m.configManager, &m.filter.ScoreMin, func() { m.rebuildRows() })
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
		_, cmd := m.sidebar.HandleKeys(m, msg)
		// When sidebar is focused, consume all keys to prevent tree navigation
		return m, cmd
	}

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Multi-select mode key handling (when active, intercept selection keys)
	if m.multiSelectMode {
		switch key {
		case "v":
			m.toggleMultiSelectMode()
			return m, nil
		case "space":
			if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) &&
				!m.rows[m.cursor].IsSectionHeader && !m.rows[m.cursor].IsSeparator && m.rows[m.cursor].task != nil {
				m.ToggleSelection(m.rows[m.cursor].task.ID)
			}
			return m, nil
		case "a":
			m.selectAllVisible()
			return m, nil
		case "A":
			m.deselectAllVisible()
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

	// Sidebar toggle
	if key == "tab" {
		task := m.SidebarSelectedTask()
		m.sidebar.CycleState(task)
		// When entering focused state, start in first non-empty linked section
		if m.sidebar.IsFocused() && task != nil {
			if proj := m.SidebarProject(); proj != nil {
				sections := sidebarGetAvailableSections(proj, task)
				if len(sections) > 0 {
					m.sidebar.SetDependencySection(sections[0])
				}
			}
		}
		return m, nil
	}

	// Prefix key activation (shared across primary views)
	// Treat both section headers and separator rows as non-task rows to prevent
	// nil dereference when accessing row.task on separator rows (cross-project mode).
	onHeader := len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) &&
		(m.rows[m.cursor].IsSectionHeader || m.rows[m.cursor].IsSeparator)
	if prefix, ok := activatePrefixKey(key, !onHeader); ok {
		m.pendingPrefix = prefix
		return m, nil
	}
	if isPrefixKeySkippedOnHeader(key, !onHeader) {
		m.setStatus("Move cursor to a task first")
		return m, nil
	}
	// Outliner-specific: P prefix for perspectives
	if key == "P" {
		m.pendingPrefix = "P"
		return m, nil
	}

	// Cross-project mode: u performs multi-project undo from crossUndoCtx.
	if model, cmd, handled := m.crossHandleUndo(key); handled {
		return model, cmd
	}

	// Shared primary view keys (view switching, project cycling, secondary views, capture, etc.)
	if cmd, handled := handleCommonPrimaryViewKey(key, m); handled {
		return m, cmd
	}

	// Cursor movement
	prevCursor := m.cursor
	if handled, cmd := handleListNavigation(key, &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash); handled {
		return m.crossSkipSeparators(prevCursor, cmd)
	}

	// Perspective cycling (h/l) — analogous to ProjectTree's view-mode cycling
	if !m.sidebar.IsFocused() {
		switch key {
		case "h", "l":
			if m.project == nil {
				return m.crossHandlePerspectiveCycle(key)
			}
			perspCount := len(PerspectivePresets())
			if key == "h" {
				m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
			} else {
				m.activePerspective = (m.activePerspective + 1) % perspCount
			}
			m.rebuildRows()
			return m, nil
		}
	}

	if model, cmd, handled := m.handleNavigationKey(key); handled {
		return model, cmd
	}

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
