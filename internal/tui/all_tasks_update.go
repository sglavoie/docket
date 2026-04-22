package tui

import (
	"fmt"

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
func (m *AllTasksViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.clearStatus()

	// Handle confirm dialog
	if m.confirm.IsActive() {
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(key); handled {
			if confirmed {
				if action == "delete-smart-list" {
					return m.handleDeleteSavedFilterTab()
				}
				if m.multiSelectMode && m.SelectedCount() > 0 {
					return m.handleBulkDelete()
				}
				return taskOpsDeleteTask(m)
			}
			return m, nil
		}
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
		case "D":
			if m.SelectedCount() > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", m.SelectedCount()))
			}
			return m, nil
		}
		// Allow prefix keys in multi-select for bulk operations
		switch key {
		case ",", "!", "d", "s", "z", "m":
			m.pendingPrefix = key
			return m, nil
		}
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, "gzF", ",!dsmct.", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, m.totalNavigableItems(), m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseAllTasksMsg{}
		}

	case "enter":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Group header navigation
	case "n":
		return m.goToNextGroup()
	case "N":
		return m.goToPrevGroup()

	// Group cycling
	case "ctrl+g":
		m.cycleGroupBy()
		return m, nil

	// Group collapse/expand
	case "tab":
		m.toggleGroupCollapse()
		return m, nil

	// Multi-select
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sort, func() {
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.setStatus("Sort reset")
		}, func(field SortField) {
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.setStatus(fmt.Sprintf("Sort: %s %s", field, m.sortDirection()))
		})
		return m, nil

	// Criteria filter: create
	case "+":
		return m, func() tea.Msg {
			return ShowFilterBuilderMsg{}
		}

	// Criteria filter: edit current tab
	case "E":
		if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
			entry := m.tabEntries[m.tabs.Active]
			if !entry.IsBuiltIn {
				sf := m.config.GetSavedFilter(entry.SavedFilterID)
				if sf != nil && sf.Mode == "criteria" {
					filter := *sf
					return m, func() tea.Msg {
						return ShowFilterEditorMsg{Filter: &filter}
					}
				}
			}
		}
		return m, nil

	// Tab reordering
	case "{":
		m.moveTabLeft()
		return m, nil
	case "}":
		m.moveTabRight()
		return m, nil

	// Smart list: delete current tab
	case "X":
		if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
			entry := m.tabEntries[m.tabs.Active]
			if !entry.IsBuiltIn {
				m.confirm.Show("delete-smart-list", fmt.Sprintf("Delete smart list %q?", entry.Name))
			}
		}
		return m, nil
	}

	// Export task's project to Markdown
	if key == "ctrl+e" {
		return m, func() tea.Msg { return ExportProjectMsg{} }
	}

	// Tab navigation
	if handleTabCycleKey(key, &m.tabs, true, func() {
		// When the new tab's IncludeCompleted setting differs from what
		// was loaded, we must reload all tasks so completed tasks are
		// included or excluded correctly.
		if m.activeTabIncludesCompleted() != m.loadedWithCompleted {
			m.loadAllTasks()
		} else {
			m.applyFiltersAndGroup()
		}
		m.clampCursor()
	}) {
		return m, nil
	}

	// Common task actions (tags, snooze, delete, edit, refresh, copy, help)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *AllTasksViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle bulk operations in multi-select mode
	if m.multiSelectMode && m.SelectedCount() > 0 {
		switch prefix {
		case ",":
			if status, ok := StatusForPrefixKey(key); ok {
				return m.handleBulkStatusChange(status)
			}
		case "!":
			if priority, ok := PriorityForPrefixKey(key); ok {
				return m.handleBulkPriorityChange(priority)
			}
		case "d":
			if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
				if isClear {
					return m.handleBulkClearDueDate()
				}
				return m.handleBulkDueDateChange(days)
			}
		case "m":
			section, ok := sectionForMoveKey(key)
			if ok {
				return m.handleBulkSectionMove(section)
			}
		}
		return m, nil
	}

	// View-specific prefixes, then common dispatch
	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		switch pfx {
		case "z":
			if k == "p" {
				m.previewMode = !m.previewMode
				return m, nil, true
			}
			handleZPrefix(k, func() {
				ExpandAllSections(m.collapsedGroups)
				m.calculateNavigation()
				m.clampCursor()
			}, func() {
				groupNames := make([]string, len(m.groups))
				for i, g := range m.groups {
					groupNames[i] = g.Name
				}
				CollapseAllSections(m.collapsedGroups, groupNames)
				m.calculateNavigation()
				if len(m.groupStarts) > 0 {
					best := m.groupStarts[0]
					for _, start := range m.groupStarts {
						if start <= m.cursor {
							best = start
						}
					}
					m.cursor = best
				}
				m.clampCursor()
			})
			return m, nil, true

		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
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

// ShouldCaptureEsc returns true when prefix mode, filters, multi-select, or confirm dialog are active.
func (m *AllTasksViewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}
