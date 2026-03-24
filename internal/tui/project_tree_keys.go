package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleFilterBarKeys handles F - filter prefix popup activation.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterBarKeys(_ string) (bool, tea.Cmd) {
	// F is now handled as a prefix key in handleKeyMsg
	return false, nil
}

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *ProjectTreeModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	// Clear active saved filter for filter changes (but not for Fv which opens saved filters)
	if key != "v" {
		m.clearActiveSavedFilter()
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
	case "D":
		return m, func() tea.Msg {
			return ShowColumnPickerMsg{Source: ViewProjectTree, ColumnConfig: m.columnConfig}
		}
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

// handleSavedFilterPrefix handles the third key after pressing Fv (saved filters sub-prefix).
func (m *ProjectTreeModel) handleSavedFilterPrefix(key string) (tea.Model, tea.Cmd) {
	cmd, _ := handleSavedFilterPrefixKey(key)
	return m, cmd
}

// handleFilterKeys handles P, S, T, N - filter operations.
// Note: 't' for type filter cycling removed - use Ft instead (t is now task prefix).
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleFilterKeys(key string) (bool, tea.Cmd) {
	switch key {
	// Note: P removed as priority filter toggle — use Fp instead.

	case "T":
		cmd, _ := handleTagKey(m, ViewProjectTree, false, m.filter.Tags)
		return true, cmd

		// Note: Due date filter cycling removed from 'u' - use Fd instead.
		// 'u' is now Vim-style undo (handled in handleMiscActionKeys).
		// Note: N removed as section filter — use F@ instead.
	}
	return false, nil
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *ProjectTreeModel) toggleSort(field SortField) {
	m.clearActiveSavedFilter()
	m.sort.Toggle(field)
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
				if section == "" {
					t.Activate()
				}
			}) {
				m.project.RelocateTaskToEndOfSection(id, section)
				count++
			}
		}
		m.clearSelection()
		m.setUndoContext(label, fmt.Sprintf("%d tasks", count))
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
		if section == "" {
			t.Activate()
		}
	})
	m.project.RelocateTaskToEndOfSection(taskID, section)
	m.setUndoContext(label, task.Title)
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// handleSortKeys handles 1-5, 0 - sort operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleSortKeys(key string) (bool, tea.Cmd) {
	handled := handleSortKey(key, &m.sort,
		func() {
			m.clearActiveSavedFilter()
			m.buildTree()
		},
		func(SortField) {
			m.clearActiveSavedFilter()
			m.sortNodes()
			m.updateVisibility()
		},
	)
	return handled, nil
}

// handleTaskCRUDKeys handles enter, n, e, D - task create/read/update/delete operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleTaskCRUDKeys(key string, visible []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "enter", "o":
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			project := m.project
			m.clearSelection()
			return true, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
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

	case "D":
		// Standalone delete with confirmation (same as tD)
		handleDeleteConfirmKey(m)
		return true, nil

	}
	return false, nil
}

// handleClipboardKeys handles y, Y, ctrl+y - clipboard operations.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleClipboardKeys(key string) (bool, tea.Cmd) {
	return handleClipboardForPrefixableView(key, m)
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
func (m *ProjectTreeModel) handleTaskAttributeKeys(key string, _ []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "*":
		cmd, _ := handleToggleActiveKey(m)
		return true, cmd
	}
	return false, nil
}

// handleMiscActionKeys handles view-specific miscellaneous action keys.
// Keys shared across primary views (u, ctrl+z, \, {, }, ctrl+r, p, S, B, w,
// ctrl+p, ctrl+k, ctrl+o, i, I, `, ~, @) are handled by handleCommonPrimaryViewKey.
// Returns true if the key was handled.
func (m *ProjectTreeModel) handleMiscActionKeys(key string, _ []*TreeNode) (bool, tea.Cmd) {
	switch key {
	case "?":
		return true, helpCmd(ViewProjectTree, "Project Tree", m.GetHelpItems())

	case "[", "]":
		tabs := m.customSectionTabs()
		if tabs != nil {
			if cycleCustomSectionTab(key, &m.customSectionTab, len(tabs), func() {
				m.updateVisibility()
				m.clampCursor()
			}) {
				return true, nil
			}
		}
		return true, nil

	case "ctrl+;":
		// Toggle focus mode (zoom into selected subtree or clear if already focused)
		visible := m.visibleNodes()
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			if m.focusedTask != nil && m.focusedTask.ID == task.ID {
				m.focusedTask = nil
			} else if task.Type.CanHaveChildren() {
				m.focusedTask = task
				m.cursor = 0
			}
			m.buildTree()
		}
		return true, nil

	case "f":
		return true, func() tea.Msg {
			return ShowProjectFocusViewMsg{Project: m.project}
		}

	case "A":
		// Archive completed tasks (only in Completed view)
		if m.viewMode == ViewModeCompleted {
			completedCount := m.project.CountCompletedTasks()
			if completedCount > 0 {
				m.confirm.Show("archive-tasks",
					fmt.Sprintf("Archive %d completed task(s) to completed.md?", completedCount))
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
	if handled, cmd := m.handleClipboardKeys(key); handled {
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
