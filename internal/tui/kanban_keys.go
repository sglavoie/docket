package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
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
		val, done, cmd := m.scoreFilterInput.HandleKey(msg)
		if done && val != nil {
			m.config.ScoreFilterCustom = val
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}
			m.filter.ScoreMin = *val
			m.loadTasks()
		}
		return m, cmd
	}

	// Handle quick add mode input first
	if m.quickAddMode {
		return m.handleQuickAddInput(msg)
	}

	// Handle confirmation
	if m.confirm.IsActive() {
		action := m.confirm.Action()
		if confirmed, handled := m.confirm.HandleKey(key); handled {
			if confirmed {
				switch action {
				case "delete":
					if m.multiSelectMode && m.selectedCount() > 0 {
						return m, m.handleBulkDelete()
					}
					return m, m.executeDelete()
				case "archive":
					return m, m.executeArchiveToggle()
				}
			}
			m.deleteTarget = nil
			m.statusMessage = ""
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
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	// Handle pending prefix keys first (two-key sequences like ",s")
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// TAB toggles focus between kanban and details panes (disabled in swimlane mode)
	if key == "tab" && !m.swimlaneEnabled() {
		m.detailsPaneFocused = !m.detailsPaneFocused
		if m.detailsPaneFocused {
			m.detailsScroll = 0 // Reset scroll when focusing details
		}
		return m, nil
	}

	// When details pane is focused, handle its specific keys
	if m.detailsPaneFocused {
		return m.handleDetailsPaneKey(key)
	}

	// Kanban pane key handling
	switch key {
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewKanban,
				ViewName:   "Kanban",
				Items:      m.GetHelpItems(),
			}
		}

	case "esc":
		return m.handleEscapeSequence()

	case "ctrl+k":
		// In swimlane mode, ctrl+k navigates to previous swimlane row
		if m.swimlaneEnabled() {
			m.moveSwimlanePrev()
			return m, nil
		}
		return m.handleEscapeSequence()

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

	// Prefix key activation for Vim-style two-key sequences
	case ",":
		m.pendingPrefix = ","
		return m, nil

	case "!":
		m.pendingPrefix = "!"
		return m, nil

	case "d":
		m.pendingPrefix = "d"
		return m, nil

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "m":
		m.pendingPrefix = "m"
		return m, nil

	case "c":
		task := m.getSelectedTask()
		if task != nil {
			m.pendingPrefix = "c"
		}
		return m, nil

	case "C":
		// Context filter (capital C, doesn't conflict with lowercase c prefix)
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}

	case "z":
		// Fold prefix for expand/collapse operations
		m.pendingPrefix = "z"
		return m, nil

	case "t":
		// Task action prefix
		m.pendingPrefix = "t"
		return m, nil

	case "b":
		// Blocker/dependency prefix
		m.pendingPrefix = "b"
		return m, nil

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
	case "enter":
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
		if m.multiSelectMode && m.selectedCount() > 0 {
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

	// Filter keybindings
	case "F":
		// Open filter prefix popup
		m.pendingPrefix = "F"
		return m, nil

	case "ctrl+f":
		// Show active filters overlay (same as project tree)
		return m, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m.projectTree}
		}

	// Tag filter selector (or bulk tag assignment in multi-select)
	case "T":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		tags := BuildTagList(m.Project.Tasks, true)
		return m, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewKanban, Title: "Filter by Tags"}
		}

	case "/":
		// Activate search mode
		m.searchMode = true
		m.searchInput.SetValue(m.filter.Search)
		m.searchInput.Focus()
		return m, textinput.Blink

	case "i":
		// Open modal quick capture view
		return m, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.Project, AllProjects: m.config.Projects}
		}

	case "y", "Y", "ctrl+y":
		if handled, cmd := handleClipboardKey(key,
			m.getSelectedTask,
			func() []*models.Task {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m.getSelectedTasksList()
				}
				return nil
			},
			func() {
				m.selectedTasks = make(map[string]bool)
				m.multiSelectMode = false
			},
		); handled {
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
			return ShowTagManagerMsg{ProjectTree: nil}
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

	// Edit last modified task
	case "ctrl+l":
		recentTasks := storage.GetRecentTasks(m.Project.Tasks, 1)
		if len(recentTasks) > 0 {
			recentTask := recentTasks[0]
			// Find the task and navigate to it
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

	// Archive/unarchive project (with confirmation)
	case "ctrl+a":
		if m.Project.Archived {
			m.statusMessage = "Unarchive project? Press 'y' to confirm, 'n' to cancel"
		} else {
			m.statusMessage = "Archive project? Press 'y' to confirm, 'n' to cancel"
		}
		m.confirm.Show("archive", m.statusMessage)
		return m, nil

	// Toggle active flag
	case "*":
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, task := range m.getSelectedTasksList() {
				task.ToggleActive()
			}
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			m.statusMessage = fmt.Sprintf("Toggled active on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		task := m.getSelectedTask()
		if task != nil {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.ToggleActive()
			m.statusMessage = fmt.Sprintf("Active: %v", task.Active)
			return m, m.saveWithStatusChange(prevStates, task)
		}
		return m, nil

	// Refresh from disk
	case "ctrl+r":
		_ = storage.LoadProject(m.Project)
		m.loadTasks()
		m.statusMessage = "Refreshed from disk"
		return m, nil

	// Tab row focus switching
	case "(", ")":
		if tabs := m.customSectionTabs(); tabs != nil {
			m.customSectionRowFocused = !m.customSectionRowFocused
		}
		return m, nil

	// Tab switching within focused row
	case "[":
		if m.customSectionRowFocused {
			if tabs := m.customSectionTabs(); tabs != nil {
				if m.customSectionTab > 0 {
					m.customSectionTab--
				} else {
					m.customSectionTab = len(tabs) - 1
				}
				m.loadTasks()
			}
		} else {
			if m.activeTab > 0 {
				m.applyTabPreset(m.activeTab - 1)
			} else {
				m.applyTabPreset(len(kanbanTabNames) - 1)
			}
		}
		return m, nil

	case "]":
		if m.customSectionRowFocused {
			if tabs := m.customSectionTabs(); tabs != nil {
				if m.customSectionTab < len(tabs)-1 {
					m.customSectionTab++
				} else {
					m.customSectionTab = 0
				}
				m.loadTasks()
			}
		} else {
			if m.activeTab < len(kanbanTabNames)-1 {
				m.applyTabPreset(m.activeTab + 1)
			} else {
				m.applyTabPreset(0)
			}
		}
		return m, nil

	// Score editor (Shift+S)
	case "S":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Display settings (Shift+O for Options)
	case "O":
		return m, func() tea.Msg {
			return ShowKanbanDisplaySettingsMsg{}
		}

	// Blockers view (capital B since lowercase b is now prefix)
	case "B":
		return m, func() tea.Msg {
			return ShowBlockersMsg{Project: m.Project}
		}

	// Priority view
	case "p":
		return m, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.Project}
		}

	// Inbox processor
	case "w":
		return m, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.Project}
		}

	// Focus shortcut (alias for zv)
	case "f":
		m.focusSubtree()
		return m, nil

	// Bulk capture
	case "I":
		return m, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.Project}
		}

	// Task jumper
	case "\\":
		return m, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.Project}
		}

	// Project cycling
	case "`":
		return m, func() tea.Msg {
			return CycleProjectMsg{Forward: true}
		}

	case "~":
		return m, func() tea.Msg {
			return CycleProjectMsg{Forward: false}
		}

	case "@":
		return m, func() tea.Msg {
			return ShowProjectCycleManagerMsg{}
		}

	// Type management: promote/demote
	case "<":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanPromote(); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.Promote()
				m.statusMessage = fmt.Sprintf("Promoted to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	case ">":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanDemote(); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.Demote()
				m.statusMessage = fmt.Sprintf("Demoted to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	// Type management: cycle leaf types (Task/Bug/Idea)
	case "H":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanCycleLeafType(true); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.CycleLeafType(true)
				m.statusMessage = fmt.Sprintf("Changed to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	case "L":
		task := m.getSelectedTask()
		if task != nil {
			if ok, _ := task.CanCycleLeafType(false); ok {
				prevStates := captureTaskStates(m.Project.Tasks)
				_ = task.CycleLeafType(false)
				m.statusMessage = fmt.Sprintf("Changed to %s", task.Type.HeadingPrefix())
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}
		return m, nil

	// Undo
	case "u":
		return m, m.performUndo()

	// Undo history
	case "ctrl+z":
		return m, func() tea.Msg {
			return ShowUndoHistoryMsg{Project: m.Project}
		}
	}

	return m, nil
}

// handleEscapeSequence handles the shared escape/exit logic for esc and ctrl+k keys.
func (m *KanbanViewModel) handleEscapeSequence() (tea.Model, tea.Cmd) {
	// Exit multi-select mode first
	if m.multiSelectMode {
		m.multiSelectMode = false
		m.selectedTasks = make(map[string]bool)
		m.statusMessage = ""
		return m, nil
	}
	// Exit completed view mode
	if m.completedViewMode {
		m.completedViewMode = false
		m.statusMessage = ""
		return m, nil
	}
	// Exit all view mode
	if m.allViewMode {
		m.allViewMode = false
		m.statusMessage = ""
		return m, nil
	}
	// Exit quick edit mode
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		m.quickEdit.Exit()
		m.statusMessage = ""
		return m, nil
	}
	// Exit focus mode if active
	if m.FocusedTask != nil {
		m.exitFocusMode()
		return m, nil
	}
	return m, func() tea.Msg {
		return CloseKanbanMsg{}
	}
}
