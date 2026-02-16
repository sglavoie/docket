package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleFilterBarKeys handles F - filter prefix popup activation.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterBarKeys(key string) (bool, tea.Cmd) {
	// F is now handled as a prefix key in handleKeyMsg
	return false, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *ProjectTreeModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Clear active saved view for filter changes (but not for Fv which opens saved views)
	if key != "v" {
		m.clearActiveSavedView()
	}

	// View-specific keys
	switch key {
	case "c":
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		} else {
			m.viewMode = ViewModeCompleted
		}
		m.applyFilterChange()
		return m, nil
	case "@":
		return m, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}
	case "d":
		m.pendingPrefix = "Fd"
		return m, nil
	case "r":
		m.filter.Actionable = !m.filter.Actionable
		m.applyFilterChange()
		return m, nil
	case "v":
		m.pendingPrefix = "Fv"
		return m, nil
	case "x":
		// Override common x to also clear view-specific filters
		clearCommonFilters(&m.filter.CommonFilterState)
		m.filter.Sections = make(map[string]bool)
		m.filter.Actionable = false
		if m.viewMode == ViewModeCompleted {
			m.viewMode = ViewModeActive
		}
		m.ClearSearch()
		m.applyFilterChange()
		return m, nil
	}

	// Common filter prefix keys
	result := handleCommonFilterPrefix(key, &m.filter.CommonFilterState, func() tea.Cmd {
		tags := BuildTagList(m.project.Tasks, false)
		return func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewProjectTree, Title: "Filter by Tags"}
		}
	})
	if cmd := applyCommonFilterResult(result, func(p string) { m.pendingPrefix = p }, m.applyFilterChange); cmd != nil {
		return m, cmd
	}
	return m, nil
}

// handleFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, Fas-Fap, FS).
func (m *ProjectTreeModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	applyFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config,
		func(p string) { m.pendingPrefix = p }, m.scoreFilterInput.Enter, m.applyFilterChange)
	return m, nil
}

// handleSavedViewPrefix handles the third key after pressing Fv (saved views sub-prefix).
func (m *ProjectTreeModel) handleSavedViewPrefix(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		// Save current view
		return m, func() tea.Msg {
			return ShowSaveViewDialogMsg{}
		}
	case "l":
		// Load/browse saved views
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	case "d":
		// Delete a saved view (browse, then Tab → Delete)
		return m, func() tea.Msg {
			return ShowCommandPaletteMsg{PreFilter: ":sv "}
		}
	}
	return m, nil
}

// handleFilterKeys handles P, S, T, N - filter operations.
// Note: 't' for type filter cycling removed - use Ft instead (t is now task prefix).
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	// Note: P removed as priority filter toggle — use Fp instead.

	case "T":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return true, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		tags := BuildTagList(m.project.Tasks, false)
		return true, func() tea.Msg {
			return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewProjectTree, Title: "Filter by Tags"}
		}

	case "N":
		return true, func() tea.Msg {
			return ShowSectionFilterSelectorMsg{ProjectTree: m}
		}

		// Note: Due date filter cycling removed from 'u' - use Fd instead.
		// 'u' is now Vim-style undo (handled in handleMiscActionKeys).
	}
	return false, nil
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *ProjectTreeModel) toggleSort(field SortField) {
	m.clearActiveSavedView()
	if m.sortField == field {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortField = field
		m.sortAscending = true
	}
	m.sortNodes()
	m.updateVisibility()
}

// sendTasksToSection moves selected tasks or cursor task to the specified section.
// Use section="" for Active. Captures task states before changes to enable
// event-based workflow triggers.
func (m *ProjectTreeModel) sendTasksToSection(section string, visible []*TreeNode) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	label := "Move to " + section
	if section == "" {
		label = "Move to Active"
	}

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for id := range m.selectedTasks {
			if m.project.MutateTask(id, func(t *models.Task) {
				t.Section = section
			}) {
				m.project.RelocateTaskToEndOfSection(id, section)
				count++
			}
		}
		m.clearSelection()
		m.pendingUndoAction = label
		m.pendingUndoContext = fmt.Sprintf("%d tasks", count)
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}

	// Single task mode
	if !CursorInBounds(m.cursor, len(visible)) {
		return false, nil
	}
	taskID := visible[m.cursor].Task.ID
	task := m.project.GetTaskByID(taskID)
	if task == nil {
		return false, nil
	}
	m.project.MutateTask(taskID, func(t *models.Task) {
		t.Section = section
	})
	m.project.RelocateTaskToEndOfSection(taskID, section)
	m.pendingUndoAction = label
	m.pendingUndoContext = task.Title
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// handleSortKeys handles 1-5, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	field, isReset, ok := SortFieldForKey(key)
	if !ok {
		return false, nil
	}
	if isReset {
		m.clearActiveSavedView()
		m.sortField = SortByDefault
		m.sortAscending = true
		m.buildTree()
	} else {
		m.toggleSort(field)
	}
	return true, nil
}

// handleTaskCRUDKeys handles enter, n, e, D - task create/read/update/delete operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskCRUDKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "enter":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			project := m.project
			m.clearSelection()
			return true, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return true, nil

	case "i":
		return true, func() tea.Msg {
			return ShowQuickCaptureMsg{Project: m.project, AllProjects: m.config.Projects}
		}

	case "I":
		return true, func() tea.Msg {
			return ShowBulkCaptureMsg{Project: m.project}
		}

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

	}
	return false, nil
}

// handleClipboardKeys handles y, Y, ctrl+y - clipboard operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleClipboardKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	return handleClipboardKey(key,
		func() *models.Task {
			if CursorInBounds(m.cursor, len(visible)) {
				return visible[m.cursor].Task
			}
			return nil
		},
		func() []*models.Task {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m.getSelectedTasks()
			}
			return nil
		},
		m.clearSelection,
	)
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

// handleTaskAttributeKeys handles *, ctrl+l - task attribute modification.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskAttributeKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
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
			m.setUndoContext("Toggle Active", task.Title)
			result := applyToggleActive(m.workflowPersistFn(), m.project, task.ID)
			if result.Err != nil {
				return true, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.buildTree()
			return true, result.Cmd
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
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab > 0 {
				m.customSectionTab--
			} else {
				m.customSectionTab = len(tabs) - 1
			}
			m.updateVisibility()
			m.clampCursor()
		}
		return true, nil

	case "]":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if m.customSectionTab < len(tabs)-1 {
				m.customSectionTab++
			} else {
				m.customSectionTab = 0
			}
			m.updateVisibility()
			m.clampCursor()
		}
		return true, nil

	case "{":
		return true, func() tea.Msg {
			return ShowNotesMsg{Project: m.project}
		}

	case "}":
		return true, func() tea.Msg {
			return ShowNoteCreatorMsg{Project: m.project}
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

	case "S":
		// Open score editor for selected task
		if len(visible) > 0 && m.cursor < len(visible) {
			task := visible[m.cursor].Task
			return true, func() tea.Msg {
				return ShowScoreEditorMsg{Task: task, Project: m.project}
			}
		}
		return true, nil

	case "f":
		return true, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "B":
		// Project blockers (capital B since lowercase b is now prefix)
		return true, func() tea.Msg {
			return ShowBlockersMsg{Project: m.project}
		}

	case "w":
		// Open inbox processor workflow
		return true, func() tea.Msg {
			return ShowInboxProcessorMsg{Project: m.project}
		}

	case "ctrl+k":
		// Open kanban board view
		return true, func() tea.Msg {
			return ShowKanbanMsg{Project: m.project}
		}

	case "ctrl+o":
		// Open outliner view
		return true, func() tea.Msg {
			return ShowOutlinerMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks (only in Completed view)
		if m.viewMode == ViewModeCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirmAction = ConfirmArchiveTasks
			}
			return true, nil
		}
		// Otherwise open all tasks view
		return true, func() tea.Msg {
			return ShowAllTasksMsg{}
		}
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
