package tui

import (
	"fmt"
	"strings"
	"time"

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

	// Handle search mode input first
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	// Handle pending prefix keys first (two-key sequences like ",s")
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// TAB toggles focus between kanban and details panes
	if key == "tab" {
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

	case "esc", "ctrl+k":
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

	case "c":
		task := m.getSelectedTask()
		if task != nil {
			m.pendingPrefix = "c"
		}
		return m, nil

	case "z":
		// Fold prefix for expand/collapse operations
		m.pendingPrefix = "z"
		return m, nil

	// Sorting keys (1-4 for sort field, 0 to reset)
	case "1":
		m.cycleSortField(SortByPriority)
		return m, nil

	case "2":
		m.cycleSortField(SortByDueDate)
		return m, nil

	case "3":
		m.cycleSortField(SortByCreatedAt)
		return m, nil

	case "4":
		m.cycleSortField(SortByStatus)
		return m, nil

	case "0":
		m.resetSort()
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

	// Open task detail view via glow
	case "enter":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: task}
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

	// Rename (inline title only)
	case "r":
		return m, m.handleRenameKey()

	// Filter keybindings
	case "F":
		// Toggle filter bar visibility
		if m.filterBar != nil {
			m.filterBar.Toggle()
		}
		return m, nil

	case "ctrl+f":
		// Show active filters overlay (same as project tree)
		return m, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m.projectTree}
		}

	case "f2":
		// Toggle high priority filter
		m.toggleKanbanPriorityFilter()
		m.loadTasks()
		return m, nil

	case "f3":
		// Cycle type filter
		m.cycleKanbanTypeFilter()
		m.loadTasks()
		return m, nil

	case "f4":
		// Cycle status filter
		m.cycleKanbanStatusFilter()
		m.loadTasks()
		return m, nil

	case "f5":
		// Tag filter selector
		return m, func() tea.Msg {
			return ShowKanbanTagFilterSelectorMsg{}
		}

	case "f1":
		// Toggle completed view
		m.toggleCompletedView()
		return m, nil

	case "f7":
		// Cycle due date filter
		m.cycleKanbanDueDateFilter()
		m.loadTasks()
		return m, nil

	case "f8":
		// Toggle actionable filter
		m.toggleActionableFilter()
		return m, nil

	// Tag filter selector (or bulk tag assignment in multi-select)
	case "T":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		return m, func() tea.Msg {
			return ShowKanbanTagFilterSelectorMsg{}
		}

	// Quick tag assignment
	case "ctrl+g":
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			project := m.Project
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			project := m.Project
			tags := append([]string{}, task.Tags...)
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        task,
					Project:     project,
					CurrentTags: tags,
				}
			}
		}
		return m, nil

	case "/":
		// Activate search mode
		m.searchMode = true
		m.searchInput.SetValue(m.filter.Search)
		m.searchInput.Focus()
		return m, textinput.Blink

	case "n":
		// Add new task with selected task as parent
		var parentTask *models.Task
		if m.FocusedTask != nil {
			parentTask = m.FocusedTask
		} else {
			parentTask = m.getSelectedTask()
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask}
		}

	case "i":
		// Open modal quick capture view
		return m, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.Project, AllProjects: m.config.Projects}
		}

	case "D":
		// Initiate delete confirmation
		if m.multiSelectMode && m.selectedCount() > 0 {
			m.statusMessage = fmt.Sprintf("Delete %d selected tasks? Press 'y' to confirm, 'n' to cancel", m.selectedCount())
			m.confirm.Show("delete", m.statusMessage)
			return m, nil
		}
		task := m.getSelectedTask()
		if task != nil {
			m.deleteTarget = task
			m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", task.ShortID)
			m.confirm.Show("delete", m.statusMessage)
		}
		return m, nil

	case "y":
		// Copy task title to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksTitleMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return m, nil

	case "Y":
		// Copy task description to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksDetailsMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return m, nil

	case "ctrl+y":
		// Copy task title and description to clipboard
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			return m, func() tea.Msg {
				return CopyTasksTitleAndDescMsg{Tasks: tasks}
			}
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: task}
			}
		}
		return m, nil

	// Comments view
	case "C":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskCommentsMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Attachments view (or select all in multi-select mode)
	case "A":
		if m.multiSelectMode {
			m.selectAllAcrossColumns()
			return m, nil
		}
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskAttachmentsMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Dependency viewer (read-only)
	case "R":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowDependencyViewerMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Dependency manager
	case "ctrl+b":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ManageDependenciesMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Section picker
	case "ctrl+n":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

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

	// Edit last modified task
	case "ctrl+l":
		recentTasks := storage.GetRecentTasks(m.Project.Tasks, 1)
		if len(recentTasks) > 0 {
			recentTask := recentTasks[0]
			// Find the task and navigate to it
			for _, section := range KanbanColumnOrder {
				visibleNodes := m.visibleNodesForSection(section)
				for i, node := range visibleNodes {
					if node.Task.ID == recentTask.ID {
						// Navigate to column and position
						for colIdx, sec := range KanbanColumnOrder {
							if sec == section {
								m.ActiveColumn = colIdx
								m.Columns[section].Cursor = i
								m.statusMessage = "Jumped to last modified"
								return m, m.handleEditTask()
							}
						}
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

	// External editor for task content
	case "ctrl+o":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: m.Project}
			}
		}
		return m, nil

	// Move task to another project
	case "M":
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: m.Project}
			}
		}
		return m, nil

	// Toggle pinned
	case "ctrl+p":
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, task := range m.getSelectedTasksList() {
				task.TogglePinned()
			}
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			m.statusMessage = fmt.Sprintf("Toggled pin on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		task := m.getSelectedTask()
		if task != nil {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.TogglePinned()
			m.statusMessage = fmt.Sprintf("Pinned: %v", task.Pinned)
			return m, m.saveWithStatusChange(prevStates, task)
		}
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

	// Tab switching
	case "[":
		if m.activeTab > 0 {
			m.applyTabPreset(m.activeTab - 1)
		} else {
			m.applyTabPreset(len(kanbanTabNames) - 1)
		}
		return m, nil

	case "]":
		if m.activeTab < len(kanbanTabNames)-1 {
			m.applyTabPreset(m.activeTab + 1)
		} else {
			m.applyTabPreset(0)
		}
		return m, nil

	// Blockers view
	case "b":
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

// handleSearchInput processes keyboard input while in search mode.
func (m *KanbanViewModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel search
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil

	case "enter":
		// Confirm search
		m.searchMode = false
		m.searchInput.Blur()
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, nil

	default:
		// Update search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Live filtering as user types
		m.filter.Search = m.searchInput.Value()
		m.loadTasks()
		return m, cmd
	}
}

// handleDetailsPaneKey handles keys when the details pane has focus.
func (m *KanbanViewModel) handleDetailsPaneKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return focus to kanban pane
		m.detailsPaneFocused = false
		return m, nil

	case "q":
		// Close the kanban view entirely
		return m, func() tea.Msg {
			return CloseKanbanMsg{}
		}

	case "enter":
		// Open task detail view via glow
		task := m.getSelectedTask()
		if task != nil {
			return m, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return m, nil

	case "j", "down":
		// Scroll description down
		task := m.getSelectedTask()
		if task != nil && task.Description != "" {
			descLines := strings.Split(task.Description, "\n")
			maxScroll := len(descLines) - 3 // Keep at least 3 lines visible
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.detailsScroll < maxScroll {
				m.detailsScroll++
			}
		}
		return m, nil

	case "k", "up":
		// Scroll description up
		if m.detailsScroll > 0 {
			m.detailsScroll--
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewKanban,
				ViewName:   "Kanban",
				Items:      m.GetHelpItems(),
			}
		}
	}

	// Other keys are no-op when details pane is focused
	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
// It clears the pending prefix and processes the combined command.
func (m *KanbanViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		// Status operations (support bulk in multi-select mode)
		switch key {
		case "s":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusInProgress)
			}
			return m, m.handleStatusStart()
		case "d":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusDone)
			}
			return m, m.handleStatusDone()
		case "b":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusBlocked)
			}
			return m, m.handleStatusBlocked()
		case "o":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusTodo)
			}
			return m, m.handleStatusOpen()
		case "x":
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(models.StatusCanceled)
			}
			return m, m.handleStatusCancelled()
		}

	case "!":
		// Priority quick actions
		switch key {
		case "1", "2", "3", "4", "5":
			priority := int(key[0] - '0')
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkPriorityChange(priority)
			}
			task := m.getSelectedTask()
			if task != nil {
				prevStates := captureTaskStates(m.Project.Tasks)
				task.Priority = priority
				task.UpdatedAt = time.Now()
				m.statusMessage = fmt.Sprintf("Priority set to %d", priority)
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}

	case "d":
		// Due date quick actions
		now := time.Now()
		var dueDate *time.Time
		var desc string

		switch key {
		case "t":
			today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
			dueDate = &today
			desc = "today"
		case "m":
			tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 0, now.Location())
			dueDate = &tomorrow
			desc = "tomorrow"
		case "w":
			nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 0, now.Location())
			dueDate = &nextWeek
			desc = "+7 days"
		case "n":
			nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 23, 59, 59, 0, now.Location())
			dueDate = &nextMonth
			desc = "+30 days"
		case "0":
			dueDate = nil
			desc = "cleared"
		case "d":
			// Open full date picker (single task only)
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, nil
			}
			task := m.getSelectedTask()
			if task == nil {
				return m, nil
			}
			project := m.Project
			return m, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			}
		default:
			return m, nil
		}

		if m.multiSelectMode && m.selectedCount() > 0 {
			return m, m.handleBulkDueDateChange(dueDate, desc)
		}
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}
		prevStates := captureTaskStates(m.Project.Tasks)
		task.DueDate = dueDate
		task.UpdatedAt = time.Now()
		m.statusMessage = fmt.Sprintf("Due date %s", desc)
		return m, m.saveWithStatusChange(prevStates, task)

	case "g":
		// Go/section navigation and send operations
		switch key {
		case "g":
			// Go to first task in current column (vim-style gg)
			if m.completedViewMode {
				m.completedCursor = 0
				return m, nil
			}
			cs := m.CurrentColumnState()
			cs.Cursor = 0
			return m, nil
		case "c":
			// Toggle completed view
			m.toggleCompletedView()
			return m, nil
		case "a":
			// Go to Active column
			m.ActiveColumn = 0
			return m, nil
		case "n":
			// Go to Next column
			m.ActiveColumn = 1
			return m, nil
		case "i":
			// Go to Inbox column
			m.ActiveColumn = 2
			return m, nil
		case "w":
			// Go to Waiting column
			m.ActiveColumn = 3
			return m, nil
		case "s":
			// Go to Someday column
			m.ActiveColumn = 4
			return m, nil
		case "A":
			// Send task(s) to Active section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionActive)
			}
			return m, m.sendTaskToSection(KanbanSectionActive)
		case "N":
			// Send task(s) to Next section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionNext)
			}
			return m, m.sendTaskToSection(KanbanSectionNext)
		case "I":
			// Send task(s) to Inbox section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionInbox)
			}
			return m, m.sendTaskToSection(KanbanSectionInbox)
		case "W":
			// Send task(s) to Waiting section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionWaiting)
			}
			return m, m.sendTaskToSection(KanbanSectionWaiting)
		case "S":
			// Send task(s) to Someday section
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkSectionMove(KanbanSectionSomeday)
			}
			return m, m.sendTaskToSection(KanbanSectionSomeday)
		case "X":
			// Deselect all in multi-select mode
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		case "h":
			// Go to task activity history
			task := m.getSelectedTask()
			if task != nil {
				project := m.Project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "c":
		// Create linked task operations
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}
		project := m.Project

		// Navigate to linked tasks
		if key == "n" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
			}
		}

		// Unlink linked task
		if key == "x" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project, ForUnlink: true}
			}
		}

		// Cross-project linking
		if key == "p" {
			return m, func() tea.Msg {
				return ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: project}
			}
		}

		var linkType LinkType
		switch key {
		case "b":
			linkType = LinkTypeBlocker
		case "d":
			linkType = LinkTypeDependent
		case "s":
			linkType = LinkTypeSubtask
		case "f":
			linkType = LinkTypeFollowUp
		default:
			return m, nil
		}
		return m, func() tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: linkType}
		}

	case "z":
		// Fold/expand operations
		switch key {
		case "a":
			// Toggle expand on current node
			m.toggleCurrentExpand()
			return m, nil
		case "o":
			// Expand current node
			m.expandCurrent()
			return m, nil
		case "c":
			// Collapse current node
			m.collapseCurrent()
			return m, nil
		case "R":
			// Expand all in column
			m.expandAllInColumn()
			return m, nil
		case "M":
			// Collapse all in column
			m.collapseAllInColumn()
			return m, nil
		case "v":
			// Focus subtree (zoom into selected task)
			m.focusSubtree()
			return m, nil
		}
	}

	return m, nil
}
