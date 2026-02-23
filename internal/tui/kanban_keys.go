package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *KanbanViewModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle quick edit mode input first
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		return m.handleQuickEditInput(msg)
	}

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, m.config, m.configManager, &m.filter.ScoreMin, m.loadTasks)
		return m, cmd
	}

	// Handle quick add mode input first
	if m.quickAddMode {
		return m.handleQuickAddInput(msg)
	}

	// Handle confirmation
	if m.confirm.IsActive() {
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(key); handled {
			if confirmed {
				switch action {
				case "delete":
					if m.multiSelectMode && m.SelectedCount() > 0 {
						return m, m.handleBulkDelete()
					}
					return m, m.executeDelete()
				case "archive":
					return m, m.executeArchiveToggle()
				case "bulk-status":
					return m, m.executePendingBulkStatus()
				}
			}
			m.deleteTarget = nil
			m.bulkOp.Clear()
			return m, nil
		}
	}

	// Handle completed view mode input
	if m.completedViewMode {
		if handled, _ := m.handleCompletedViewKey(key); handled {
			// Check for pending prefix from completed view
			if m.pendingPrefix != "" {
				return m, nil
			}
			return m, nil
		}
	}

	// Handle all view mode input
	if m.allViewMode {
		if handled, _ := m.handleAllViewKey(key); handled {
			// Check for pending prefix from all view
			if m.pendingPrefix != "" {
				return m, nil
			}
			return m, nil
		}
	}

	// Handle search mode input first
	if m.IsSearchMode() {
		return m.handleSearchInput(msg)
	}

	// Handle pending prefix keys first (two-key sequences like ",s")
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// TAB toggles focus between kanban and details panes (disabled in swimlane mode)
	if key == "tab" && !m.swimlaneEnabled() {
		m.detailsPane.Focused = !m.detailsPane.Focused
		if m.detailsPane.Focused {
			m.detailsPane.ResetScroll()
		}
		return m, nil
	}

	// When details pane is focused, handle its specific keys
	if m.detailsPane.Focused {
		return m.handleDetailsPaneKey(key)
	}

	// Kanban-specific overrides before shared dispatcher
	switch key {
	case "ctrl+k":
		// In swimlane mode, ctrl+k navigates to previous swimlane row
		if m.swimlaneEnabled() {
			m.moveSwimlanePrev()
			return m, nil
		}
		// Otherwise fall through to shared dispatcher (view switching)

	case "ctrl+l":
		// Kanban-specific: navigate to task then edit
		recentTasks := storage.GetRecentTasks(m.Project.Tasks, 1)
		if len(recentTasks) > 0 {
			recentTask := recentTasks[0]
			for colIdx, def := range m.columnDefs {
				visibleNodes := m.visibleNodesForSection(def.Key)
				for i, node := range visibleNodes {
					if node.Task.ID == recentTask.ID {
						m.ActiveColumn = colIdx
						m.Columns[def.Key].Cursor = i
						m.statusMessage = "Jumped to last modified"
						return m, m.handleEditTask()
					}
				}
			}
			m.statusMessage = "Last modified task not visible"
		} else {
			m.statusMessage = "No recently modified task"
		}
		return m, nil
	}

	// Shared primary view keys (view switching, project cycling, secondary views, capture, etc.)
	if cmd, handled := handleCommonPrimaryViewKey(key, m); handled {
		return m, cmd
	}

	// Prefix key activation (shared across primary views)
	if prefix, ok := activatePrefixKey(key, m.getSelectedTask() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Kanban pane key handling
	switch key {
	case "?":
		return m, helpCmd(ViewKanban, "Kanban", m.GetHelpItems())

	case "esc":
		RunEscCascade(m.escSteps())
		return m, nil

	case "ctrl+j":
		// In swimlane mode, ctrl+j navigates to next swimlane row
		if m.swimlaneEnabled() {
			m.moveSwimlaneNext()
			return m, nil
		}
		return m, nil

	case "ctrl+h":
		m.cycleSwimlaneBy()
		return m, nil

	case "q":
		// Exit focus mode first if active
		if m.FocusedTask != nil {
			m.exitFocusMode()
			return m, nil
		}
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	// Vertical navigation within current column
	case "up", "k":
		m.moveCursorUp()
		return m, nil

	case "down", "j":
		m.moveCursorDown()
		return m, nil

	// Horizontal navigation between columns
	case "h":
		m.moveColumnLeft()
		return m, nil

	case "l":
		m.moveColumnRight()
		return m, nil

	// Indent/outdent (hierarchy adjustment)
	case "left":
		return m, m.outdentTask()

	case "right":
		return m, m.indentTask()

	// Jump to last task in column
	case "G":
		// Go to last visible node in current column
		visibleCount := m.currentVisibleNodeCount()
		if visibleCount > 0 {
			cs := m.CurrentColumnState()
			cs.Cursor = visibleCount - 1
		}
		return m, nil

	// Task movement between columns (use {/} to free H/L for type cycling)
	case "{":
		// Move selected task to the left column
		return m, m.moveTaskLeft()

	case "}":
		// Move selected task to the right column
		return m, m.moveTaskRight()

	// Task reordering within column
	case "J":
		// Swap selected task with task below
		return m, m.swapTaskDown()

	case "K":
		// Swap selected task with task above
		return m, m.swapTaskUp()

	case "C":
		// Context filter (capital C, doesn't conflict with lowercase c prefix)
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}

	// Sorting keys (1-5 for sort field, 0 to reset)
	case "1", "2", "3", "4", "5", "0":
		if field, isReset, ok := SortFieldForKey(key); ok {
			if isReset {
				m.resetSort()
			} else {
				m.cycleSortField(field)
			}
		}
		return m, nil

	// Multi-select mode
	case "v":
		m.toggleMultiSelectMode()
		return m, nil

	case " ":
		// Space: toggle selection in multi-select mode
		if m.multiSelectMode {
			m.toggleTaskSelection()
		}
		return m, nil

	case "a":
		// Select all in column (only in multi-select mode)
		if m.multiSelectMode {
			m.selectAllInColumn()
		}
		return m, nil

	// Open task detail view
	case "enter", "o":
		task := m.getSelectedTask()
		if task != nil {
			project := m.Project
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	// Edit task in configured editor
	case "e":
		if m.multiSelectMode && m.SelectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   tasks,
					Project: project,
				}
			}
		}
		return m, m.handleEditTask()

	// Quick edit (inline with modifiers)
	case "E":
		return m, m.handleQuickEditKey()

	case "ctrl+f":
		// Show active filters overlay (same as project tree)
		return m, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m.projectTree}
		}

	// Tag filter selector (or bulk tag assignment in multi-select)
	case "T":
		cmd, _ := handleTagKey(m, ViewKanban, true, m.filter.Tags)
		return m, cmd

	case "/":
		// Activate search mode (pre-populate with existing filter search)
		m.searchInput.SetValue(m.filter.Search)
		cmd := m.ActivateSearch()
		return m, cmd

	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardForPrefixableView(key, m); handled {
			return m, cmd
		}
		return m, nil

	// Select all in multi-select mode, or open all tasks view
	case "A":
		if m.multiSelectMode {
			m.selectAllAcrossColumns()
			return m, nil
		}
		return m, func() tea.Msg {
			return ShowAllTasksMsg{}
		}

	// Tag manager
	case "ctrl+t":
		return m, func() tea.Msg {
			return ShowTagManagerMsg{Project: m.Project}
		}

	// Section manager
	case "ctrl+s":
		return m, func() tea.Msg {
			return ShowSectionManagerMsg{Project: m.Project}
		}

	// Cycle group-by mode
	case "ctrl+g":
		m.cycleGroupBy()
		return m, nil

	// Archive/unarchive project (with confirmation)
	case "ctrl+a":
		if m.Project.Archived {
			m.statusMessage = "Unarchive project? (y/n)"
		} else {
			m.statusMessage = "Archive project? (y/n)"
		}
		m.confirm.Show("archive", m.statusMessage)
		return m, nil

	// Toggle active flag
	case "*":
		cmd, _ := handleToggleActiveKey(m)
		return m, cmd

	// Tab row focus switching
	case "(", ")":
		if tabs := m.customSectionTabs(); tabs != nil {
			m.customSectionRowFocused = !m.customSectionRowFocused
		}
		return m, nil

	// Tab switching within focused row
	case "[", "]":
		if m.customSectionRowFocused {
			if tabs := m.customSectionTabs(); tabs != nil {
				cycleCustomSectionTab(key, &m.customSectionTab, len(tabs), m.loadTasks)
			}
		} else {
			// Cycle preset tabs with wraparound
			cycleCustomSectionTab(key, &m.tabs.Active, len(kanbanTabNames), func() {
				m.applyTabPreset(m.tabs.Active)
			})
		}
		return m, nil

	// Display settings (Shift+O for Options)
	case "O":
		return m, func() tea.Msg {
			return ShowKanbanDisplaySettingsMsg{}
		}

	// Focus shortcut (alias for zv)
	case "f":
		m.focusSubtree()
		return m, nil

	// Type management: promote/demote/cycle leaf types
	case "<", ">", "H", "L":
		cmd, _ := handleTypeChangeKey(key, m)
		return m, cmd

	}

	return m, nil
}

// escSteps defines the Esc dismissal cascade for Kanban:
// multi-select → completed view → all view → quick edit → focus mode.
func (m *KanbanViewModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.multiSelectMode }, Clear: func() {
			m.ExitMultiSelect()
			m.statusMessage = ""
		}},
		{Active: func() bool { return m.completedViewMode }, Clear: func() {
			m.completedViewMode = false
			m.statusMessage = ""
		}},
		{Active: func() bool { return m.allViewMode }, Clear: func() {
			m.allViewMode = false
			m.statusMessage = ""
		}},
		{Active: func() bool { return m.quickEdit != nil && m.quickEdit.IsActive() }, Clear: func() {
			m.quickEdit.Exit()
			m.statusMessage = ""
		}},
		{Active: func() bool { return m.FocusedTask != nil }, Clear: func() {
			m.exitFocusMode()
		}},
	}
}
