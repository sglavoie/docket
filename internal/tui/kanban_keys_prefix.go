package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

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
		if status, ok := StatusForPrefixKey(key); ok {
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkStatusChange(status)
			}
			return m, m.handleStatusTransition(status)
		}

	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkPriorityChange(priority)
			}
			task := m.getSelectedTask()
			if task != nil {
				result := applyPriorityChange(m.workflowPersistFn(), m.Project, task.ID, priority)
				if result.Err != nil {
					m.statusMessage = result.Err.Error()
					return m, nil
				}
				if result.StatusMessage != "" {
					m.statusMessage = result.StatusMessage
				}
				m.loadTasks()
				return m, result.Cmd
			}
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if m.multiSelectMode && m.selectedCount() > 0 {
				dueDate, desc := dueDateFromDays(days, isClear)
				return m, m.handleBulkDueDateChange(dueDate, desc)
			}
			task := m.getSelectedTask()
			if task == nil {
				return m, nil
			}
			var result TaskOpResult
			if isClear {
				result = applyClearDueDate(m.workflowPersistFn(), m.Project, task.ID)
			} else {
				result = applyQuickDueDate(m.workflowPersistFn(), m.Project, task.ID, days)
			}
			if result.Err != nil {
				m.statusMessage = result.Err.Error()
				return m, nil
			}
			if result.StatusMessage != "" {
				m.statusMessage = result.StatusMessage
			}
			m.loadTasks()
			return m, result.Cmd
		}
		if key == "d" {
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
		}

	case "g":
		// Section column jump (a, n, i, w, s, r)
		if section, ok := sectionForMoveKey(key); ok {
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(section); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		}

		// Go/navigation operations
		switch key {
		case "g":
			// Go to first task in current column (vim-style gg)
			if m.completedViewMode {
				m.completedCursor = 0
				return m, nil
			}
			if m.allViewMode {
				m.allCursor = 0
				return m, nil
			}
			cs := m.CurrentColumnState()
			cs.Cursor = 0
			return m, nil
		case "c":
			// Toggle completed view
			m.toggleCompletedView()
			return m, nil
		case "*":
			// Toggle all view
			m.toggleAllView()
			return m, nil
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

	case "m":
		// Section move shortcuts (a, n, i, w, s, r) — only in section mode
		if section, ok := sectionForMoveKey(key); ok {
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(section)
				}
				return m, m.sendTaskToSection(section)
			}
			return m, nil
		}

		// Move operations — mK/mJ always work
		switch key {
		case "K":
			// Move task to top of column
			return m, m.moveTaskToTop()
		case "J":
			// Move task to bottom of column
			return m, m.moveTaskToBottom()
		}

	case "c":
		task := m.getSelectedTask()
		if task == nil {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, task, m.Project, true, true); handled {
			return m, cmd
		}
		return m, nil

	case "F":
		// Filter prefix - first level
		return m.handleKanbanFilterPrefix(key)

	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleKanbanFilterSubPrefix(prefix, key)

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

	case "t":
		// Task action prefix
		return m.handleKanbanTaskPrefix(key)

	case "b":
		// Blocker/dependency prefix
		return m.handleKanbanBlockerPrefix(key)
	}

	return m, nil
}

// handleKanbanTaskPrefix handles the second key after pressing t (task action prefix).
func (m *KanbanViewModel) handleKanbanTaskPrefix(key string) (tea.Model, tea.Cmd) {
	task := m.getSelectedTask()
	if task == nil && key != "n" && key != "N" {
		return m, nil
	}
	project := m.Project

	// Multi-select getter for shared handlers
	getMultiTasks := func() []*models.Task {
		if m.multiSelectMode && m.selectedCount() > 0 {
			return m.getSelectedTasksList()
		}
		return nil
	}

	// Try shared message-only handlers first (to, tc, ta, tl, td, tm)
	if task != nil {
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return m, cmd
		}
	}

	// Try shared action handlers (tn, tN, ts, tt)
	if handled, cmd := handleTaskActionPrefixKey(key, task, project, m.FocusedTask, getMultiTasks); handled {
		return m, cmd
	}

	switch key {
	case "r":
		if task != nil {
			return m, m.handleRenameKey()
		}
		return m, nil
	case "p":
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, t := range m.getSelectedTasksList() {
				t.TogglePinned()
			}
			m.ExitMultiSelect()
			m.statusMessage = fmt.Sprintf("Toggled pin on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		if task != nil {
			result := applyTogglePinned(m.workflowPersistFn(), m.Project, task.ID)
			if result.Err != nil {
				m.statusMessage = result.Err.Error()
				return m, nil
			}
			if result.StatusMessage != "" {
				m.statusMessage = result.StatusMessage
			}
			m.loadTasks()
			return m, result.Cmd
		}
		return m, nil
	case "D":
		if m.multiSelectMode && m.selectedCount() > 0 {
			m.statusMessage = fmt.Sprintf("Delete %d selected tasks? (y/n)", m.selectedCount())
			m.confirm.Show("delete", m.statusMessage)
			return m, nil
		}
		if task != nil {
			m.deleteTarget = task
			m.statusMessage = fmt.Sprintf("Delete task #%d? (y/n)", task.ShortID)
			m.confirm.Show("delete", m.statusMessage)
		}
		return m, nil
	}
	return m, nil
}

// handleKanbanBlockerPrefix handles the second key after pressing b (blocker/dependency prefix).
func (m *KanbanViewModel) handleKanbanBlockerPrefix(key string) (tea.Model, tea.Cmd) {
	task := m.getSelectedTask()
	if task == nil {
		return m, nil
	}
	if handled, cmd := handleBlockerPrefixKey(key, task, m.Project); handled {
		return m, cmd
	}
	return m, nil
}
