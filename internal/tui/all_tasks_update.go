package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *AllTasksViewModel) Init() tea.Cmd {
	return nil
}

func (m *AllTasksViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, m.SetSize, m.loadAllTasks, m.clampCursor, func(s string) { m.statusMessage = s }); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
func (m *AllTasksViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.err = nil
	m.statusMessage = ""

	// Handle confirm dialog
	if m.confirm.IsActive() {
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(key); handled {
			if confirmed {
				if action == "delete-smart-list" {
					return m.handleDeleteSmartList()
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
		case " ":
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
		case ",", "!", "d", "z":
			m.pendingPrefix = key
			return m, nil
		case "m":
			m.pendingPrefix = "m"
			return m, nil
		}
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, ",!dgzF", "mc", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Kanban-specific navigation (intercept before list navigation and tab cycling)
	if m.layoutMode == ATLayoutKanban {
		if model, cmd, handled := m.handleKanbanKeyMsg(key); handled {
			return model, cmd
		}
	}

	// Navigation (list mode only — kanban intercepts j/k/G/etc. above)
	if m.layoutMode == ATLayoutList {
		if handled, cmd := handleListNavigation(key, &m.cursor, m.totalNavigableItems(), m.Height(), &m.cursorFlash); handled {
			return m, cmd
		}
	}

	switch key {
	case "ctrl+k":
		m.toggleLayout()
		return m, nil

	case "esc", "q":
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

	// Group header navigation (list mode only)
	case "n":
		if m.layoutMode == ATLayoutList {
			return m.goToNextGroup()
		}
	case "N":
		if m.layoutMode == ATLayoutList {
			return m.goToPrevGroup()
		}

	// Group cycling
	case "ctrl+g":
		m.cycleGroupBy()
		if m.layoutMode == ATLayoutKanban {
			// Flat grouping is degenerate in kanban — auto-switch
			if m.groupBy == ATGroupByFlat {
				m.groupBy = ATGroupByProject
				m.applyFiltersAndGroup()
				m.clampCursor()
				m.statusMessage = "Switched to Project grouping for kanban"
			}
			m.reinitKanbanColumns()
			m.ensureActiveKanbanColumnVisible()
		}
		return m, nil

	// Group collapse/expand (list mode only)
	case "tab":
		if m.layoutMode == ATLayoutList {
			m.toggleGroupCollapse()
		}
		return m, nil

	// Multi-select
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	// Toggle active flag
	case "*":
		return taskOpsToggleActive(m)

	// Sort keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		handleSortKey(key, &m.sort, func() {
			m.applyFiltersAndGroup()
			m.clampCursor()
			if m.layoutMode == ATLayoutKanban {
				m.reinitKanbanColumns()
			}
			m.statusMessage = "Sort reset"
		}, func(field SortField) {
			m.applyFiltersAndGroup()
			m.clampCursor()
			if m.layoutMode == ATLayoutKanban {
				m.reinitKanbanColumns()
			}
			m.statusMessage = fmt.Sprintf("Sort: %s %s", field, m.sortDirection())
		})
		return m, nil

	// Score editor
	case "S":
		if item := m.getTaskAtCursor(); item != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: item.Task, Project: item.Project}
			}
		}
		return m, nil

	// Smart list: create
	case "+":
		return m, func() tea.Msg {
			return ShowSmartListBuilderMsg{}
		}

	// Smart list: edit current tab
	case "E":
		if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
			entry := m.tabEntries[m.tabs.Active]
			if !entry.IsBuiltIn {
				sl := m.config.GetSmartList(entry.SmartListID)
				if sl != nil {
					return m, func() tea.Msg {
						return ShowSmartListEditorMsg{SmartList: sl}
					}
				}
			}
		}
		return m, nil

	// Tab reordering (list mode only — kanban uses {/} for task movement)
	case "{":
		if m.layoutMode == ATLayoutList {
			m.moveTabLeft()
			return m, nil
		}
	case "}":
		if m.layoutMode == ATLayoutList {
			m.moveTabRight()
			return m, nil
		}

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

	// Tab navigation — in kanban mode, only [/] cycle tabs (h/l are column nav)
	if handleTabCycleKey(key, &m.tabs, m.layoutMode == ATLayoutList, func() {
		m.applyFiltersAndGroup()
		m.clampCursor()
		if m.layoutMode == ATLayoutKanban {
			m.reinitKanbanColumns()
			m.ensureActiveKanbanColumnVisible()
		}
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

		case "!":
			if _, ok := PriorityForPrefixKey(k); ok {
				model, cmd := taskOpsChangePriority(m, k)
				return model, cmd, true
			}
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
			m.statusMessage = "Filters cleared"
		}),
	}
}

// ShouldCaptureEsc returns true when prefix mode, filters, multi-select, or confirm dialog are active.
func (m *AllTasksViewModel) ShouldCaptureEsc() bool {
	return SecondaryEscCapture(&m.InteractiveViewState, m.escSteps())
}
