package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleFilterKeys handles d, P, t, S, T, N, u - filter operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "d":
		m.showCompleted = !m.showCompleted
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "P":
		if m.filterPriority == 0 {
			m.filterPriority = 1
		} else {
			m.filterPriority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "t":
		m.filterType = cycleFilter(m.filterType, models.AllTaskTypes())
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "S":
		m.filterStatus = cycleFilter(m.filterStatus, models.AllTaskStatuses())
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.tagInputMode = true
			m.tagInput.Focus()
			return true, textinput.Blink
		}
		return true, func() tea.Msg {
			return ShowTagFilterSelectorMsg{ProjectTree: m}
		}

	case "N":
		return true, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}

	case "u":
		m.filterDueDate = (m.filterDueDate + 1) % 5
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	}
	return false, nil
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *ProjectTreeModel) toggleSort(field SortField) {
	if m.sortField == field {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortField = field
		m.sortAscending = true
	}
	m.sortNodes()
	m.updateVisibility()
}

// handleSortKeys handles 1, 2, 3, 4, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "1":
		m.toggleSort(SortByPriority)
		return true, nil
	case "2":
		m.toggleSort(SortByDueDate)
		return true, nil
	case "3":
		m.toggleSort(SortByCreatedAt)
		return true, nil
	case "4":
		m.toggleSort(SortByStatus)
		return true, nil
	case "0":
		m.sortField = SortByDefault
		m.sortAscending = true
		m.buildTree()
		return true, nil
	}
	return false, nil
}

// handleTaskCRUDKeys handles enter, n, e, D - task create/read/update/delete operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskCRUDKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "enter":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.clearSelection()
			return true, func() tea.Msg {
				return GlowViewMsg{Task: task}
			}
		}
		return true, nil

	case "n":
		var parentTask *models.Task
		if m.focusedTask != nil {
			// In focus mode, default to focused task as parent
			parentTask = m.focusedTask
		} else if CursorInBounds(m.cursor, len(visible)) {
			parentTask = visible[m.cursor].Task
		}
		return true, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask}
		}

	case "i":
		return true, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.project}
		}

	case "I":
		return true, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.project}
		}

	case "O":
		// Create a new note (Shift+O)
		var parentTask *models.Task
		if m.focusedTask != nil {
			// In focus mode, default to focused task as parent
			parentTask = m.focusedTask
		} else if CursorInBounds(m.cursor, len(visible)) {
			parentTask = visible[m.cursor].Task
		}
		return true, func() tea.Msg {
			return CreateNoteMsg{ParentTask: parentTask}
		}

	case "e":
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: visible[m.cursor].Task}
			}
		}
		return true, nil

	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirmAction = "bulk-delete"
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			m.confirmAction = "delete"
			m.deleteTarget = visible[m.cursor].Task
		}
		return true, nil
	}
	return false, nil
}

// handleClipboardKeys handles y, Y - clipboard operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleClipboardKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksTitleMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskTitleMsg{Task: task}
			}
		}
		return true, nil

	case "Y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksDetailsMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskDetailsMsg{Task: task}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleExpandCollapseKeys handles E, c - expand/collapse all operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleExpandCollapseKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "E":
		for _, node := range m.nodes {
			node.Expanded = true
		}
		m.updateVisibility()
		return true, nil

	case "c":
		for _, node := range m.nodes {
			if node.Level > 0 {
				node.Expanded = false
			}
		}
		m.updateVisibility()
		return true, nil
	}
	return false, nil
}

// handleFilterOverlayKeys handles ctrl+f, ctrl+t, ctrl+s - filter/manager overlay operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterOverlayKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "ctrl+f":
		return true, func() tea.Msg {
			return ShowFilterMsg{ProjectTree: m}
		}

	case "ctrl+t":
		return true, func() tea.Msg {
			return ShowTagManagerMsg{ProjectTree: m}
		}

	case "ctrl+s":
		return true, func() tea.Msg {
			return ShowSectionManagerMsg{ProjectTree: m}
		}
	}
	return false, nil
}

// handleTaskAttributeKeys handles ctrl+d, ctrl+n, B, ctrl+p, ctrl+o - task attribute modification.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskAttributeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "ctrl+d":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+n":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "B":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowDependencyViewerMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+b":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ManageDependenciesMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "ctrl+p":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.TogglePinned()
			}
			m.clearSelection()
			m.setUndoContext("Toggle Pin", fmt.Sprintf("%d tasks", count))
			return true, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.TogglePinned()
			m.setUndoContext("Toggle Pin", task.Title)
			return true, m.saveAndRefresh()
		}
		return true, nil

	case "*":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.ToggleActive()
			}
			m.clearSelection()
			m.setUndoContext("Toggle Active", fmt.Sprintf("%d tasks", count))
			return true, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.ToggleActive()
			m.setUndoContext("Toggle Active", task.Title)
			return true, m.saveAndRefresh()
		}
		return true, nil

	case "ctrl+o":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: m.project}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleMiscActionKeys handles z, R, ?, \, r, p - miscellaneous action operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMiscActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "z":
		// Quick single-step undo using legacy backup
		return true, m.performUndo()

	case "ctrl+z":
		// Open undo history view
		return true, func() tea.Msg {
			return ShowUndoHistoryMsg{Project: m.project}
		}

	case "?":
		return true, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewProjectTree,
				ViewName:   "Project Tree",
				Items:      m.GetHelpItems(),
			}
		}

	case "\\":
		return true, func() tea.Msg {
			return ShowTaskJumperMsg{Project: m.project}
		}

	case "r":
		m.clearSelection()
		storage.LoadProject(m.project)
		m.buildTree()
		return true, nil

	case "p":
		return true, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.project}
		}

	case "w":
		// Open inbox processor workflow
		inboxTasks := m.getInboxTasksForProcessing()
		if len(inboxTasks) > 0 {
			return true, func() tea.Msg {
				return ShowInboxProcessorMsg{Project: m.project}
			}
		}
		// No tasks in inbox - do nothing
		return true, nil
	}
	return false, nil
}

// handleActionKeys routes action keys to specialized sub-handlers.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleActionKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	if handled, cmd := m.handleTaskCRUDKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleClipboardKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleExpandCollapseKeys(key); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleFilterOverlayKeys(key); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleTaskAttributeKeys(key, visible); handled {
		return handled, cmd
	}
	if handled, cmd := m.handleMiscActionKeys(key, visible); handled {
		return handled, cmd
	}
	return false, nil
}
