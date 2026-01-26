package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleFilterBarKeys handles F, F1-F7 - filter bar toggle and quick filter operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterBarKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "F":
		m.filterBar.SetEnabled(!m.filterBar.IsEnabled())
		return true, nil
	case "f1":
		// F1 now switches to Completed view (since completed tasks have their own view)
		m.viewMode = ViewModeCompleted
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	case "f2":
		if m.filter.Priority == 0 {
			m.filter.Priority = 1
		} else {
			m.filter.Priority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	case "f3":
		m.filter.Type = cycleFilter(m.filter.Type, models.AllTaskTypes())
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	case "f4":
		m.filter.Status = cycleFilter(m.filter.Status, models.AllTaskStatuses())
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	case "f5":
		return true, func() tea.Msg {
			return ShowTagFilterSelectorMsg{ProjectTree: m}
		}
	case "f6":
		return true, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}
	case "f7":
		m.filter.DueDate = (m.filter.DueDate + 1) % 5
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	case "f8":
		m.filter.Actionable = !m.filter.Actionable
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil
	}
	return false, nil
}

// handleFilterKeys handles P, t, S, T, N, u - filter operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	case "P":
		if m.filter.Priority == 0 {
			m.filter.Priority = 1
		} else {
			m.filter.Priority = 0
		}
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "t":
		m.filter.Type = cycleFilter(m.filter.Type, models.AllTaskTypes())
		m.updateVisibility()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return true, nil

	case "S":
		m.filter.Status = cycleFilter(m.filter.Status, models.AllTaskStatuses())
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

		// Note: Due date filter cycling removed from 'u' - use F7 instead.
		// 'u' is now Vim-style undo (handled in handleMiscActionKeys).
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

// sendTasksToActive moves selected tasks or cursor task to the Active view (clears section).
// Captures task states before changes to enable event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToActive(visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if task := m.project.GetTaskByID(id); task != nil {
				task.Section = ""
				task.UpdatedAt = time.Now()
				count++
			}
		}
		m.clearSelection()
		m.pendingUndoAction = "Move to Active"
		m.pendingUndoContext = fmt.Sprintf("%d tasks", count)
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	task := visible[m.cursor].Task
	task.Section = ""
	task.UpdatedAt = time.Now()
	m.pendingUndoAction = "Move to Active"
	m.pendingUndoContext = task.Title
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// sendTasksToSection moves selected tasks or cursor task to the specified section.
// Captures task states before changes to enable event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToSection(section string, visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if task := m.project.GetTaskByID(id); task != nil {
				task.Section = section
				task.UpdatedAt = time.Now()
				count++
			}
		}
		m.clearSelection()
		m.pendingUndoAction = "Move to " + section
		m.pendingUndoContext = fmt.Sprintf("%d tasks", count)
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	task := visible[m.cursor].Task
	task.Section = section
	task.UpdatedAt = time.Now()
	m.pendingUndoAction = "Move to " + section
	m.pendingUndoContext = task.Title
	return true, m.saveAndRefreshWithPrevStates(prevStates)
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

	case "o":
		// Create a new note
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

	case "O":
		// Turn task into note (requires confirmation)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if allowed, reason := task.CanTurnIntoNote(); allowed {
				m.confirmAction = ConfirmTurnIntoNote
				m.noteTarget = task
			} else {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("cannot convert to note: %s", reason)}
				}
			}
		}
		return true, nil

	case "e":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return true, func() tea.Msg {
				return ShowBulkEditMsg{
					Tasks:   m.getSelectedTasks(),
					Project: m.project,
				}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: visible[m.cursor].Task}
			}
		}
		return true, nil

	case "E":
		// Quick edit mode: inline editing with modifier syntax
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.enterQuickEditMode(task, false)
		}
		return true, nil

	case "r":
		// Rename mode: inline title editing only (no modifier parsing)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.enterQuickEditMode(task, true)
		}
		return true, nil

	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirmAction = ConfirmBulkDelete
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if task.IsRecurring() {
				m.confirmAction = ConfirmDeleteRecurring
			} else {
				m.confirmAction = ConfirmDelete
			}
			m.deleteTarget = task
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

	case "ctrl+y":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			m.clearSelection()
			return true, func() tea.Msg {
				return CopyTasksTitleAndDescMsg{Tasks: tasks}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return CopyTaskTitleAndDescMsg{Task: task}
			}
		}
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

	case "R":
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

	case "c":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowTaskCommentsMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "A":
		// In Completed view, 'A' is handled by handleMiscActionKeys for archiving
		// In other views, 'A' opens attachments
		if m.viewMode != ViewModeCompleted && CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowTaskAttachmentsMsg{Task: task, Project: m.project}
			}
		}
		// Fall through to let handleMiscActionKeys handle archive in Completed view
		return false, nil

	// Note: "C" (completed view) is now handled via "g" prefix (gc)

	case "ctrl+g":
		// Quick tag assignment
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return true, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        task,
					Project:     m.project,
					CurrentTags: append([]string{}, task.Tags...),
				}
			}
		}
		return true, nil

	case "ctrl+l":
		// Edit last modified task
		recentTasks := storage.GetRecentTasks(m.project.Tasks, 1)
		if len(recentTasks) > 0 {
			return true, func() tea.Msg {
				return EditTaskMsg{Task: recentTasks[0]}
			}
		}
		return true, nil

	case "M":
		// Move task to another project
		// Not available in multi-select mode (too complex with subtrees)
		if m.multiSelectMode && len(m.selectedTasks) > 1 {
			return true, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{
					Task:          task,
					SourceProject: m.project,
				}
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
	case "u":
		// Undo last change (Vim-style)
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

	case "[":
		return true, func() tea.Msg {
			return ShowScratchpadMsg{Project: m.project}
		}

	case "]":
		return true, func() tea.Msg {
			return EditScratchpadMsg{Project: m.project}
		}

	case "ctrl+r":
		m.clearSelection()
		_ = storage.LoadProject(m.project)
		m.buildTree()
		return true, nil

	case "p":
		return true, func() tea.Msg {
			return ShowPriorityViewMsg{Project: m.project}
		}

	case "f":
		return true, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "b":
		return true, func() tea.Msg {
			return ShowBlockersMsg{Project: m.project}
		}

	case "w":
		// Open inbox processor workflow
		return true, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks (only in Completed view)
		if m.viewMode == ViewModeCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirmAction = ConfirmArchiveTasks
			}
		}
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
