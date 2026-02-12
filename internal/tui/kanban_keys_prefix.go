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
				prevStates := captureTaskStates(m.Project.Tasks)
				m.Project.MutateTask(task.ID, func(t *models.Task) {
					t.Priority = priority
				})
				m.statusMessage = fmt.Sprintf("Priority set to %d", priority)
				return m, m.saveWithStatusChange(prevStates, task)
			}
		}

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			dueDate, desc := dueDateFromDays(days, isClear)
			if m.multiSelectMode && m.selectedCount() > 0 {
				return m, m.handleBulkDueDateChange(dueDate, desc)
			}
			task := m.getSelectedTask()
			if task == nil {
				return m, nil
			}
			prevStates := captureTaskStates(m.Project.Tasks)
			m.Project.MutateTask(task.ID, func(t *models.Task) {
				t.DueDate = dueDate
			})
			m.statusMessage = fmt.Sprintf("Due date %s", desc)
			return m, m.saveWithStatusChange(prevStates, task)
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
		case "a":
			// Go to Active column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionActive); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		case "n":
			// Go to Next column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionNext); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		case "i":
			// Go to Inbox column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionInbox); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		case "w":
			// Go to Waiting column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionWaiting); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		case "r":
			// Go to Review column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionReview); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
			return m, nil
		case "s":
			// Go to Someday column (section mode only)
			if IsSectionGroupBy(m.groupBy) {
				if idx := m.columnIndexForKey(KanbanSectionSomeday); idx >= 0 {
					m.ActiveColumn = idx
					m.ensureActiveColumnVisible()
				}
			}
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
		// Move operations — section shortcuts only in section mode, mK/mJ always work
		switch key {
		case "a":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionActive)
				}
				return m, m.sendTaskToSection(KanbanSectionActive)
			}
			return m, nil
		case "n":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionNext)
				}
				return m, m.sendTaskToSection(KanbanSectionNext)
			}
			return m, nil
		case "i":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionInbox)
				}
				return m, m.sendTaskToSection(KanbanSectionInbox)
			}
			return m, nil
		case "w":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionWaiting)
				}
				return m, m.sendTaskToSection(KanbanSectionWaiting)
			}
			return m, nil
		case "r":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionReview)
				}
				return m, m.sendTaskToSection(KanbanSectionReview)
			}
			return m, nil
		case "s":
			if IsSectionGroupBy(m.groupBy) {
				if m.multiSelectMode && m.selectedCount() > 0 {
					return m, m.handleBulkSectionMove(KanbanSectionSomeday)
				}
				return m, m.sendTaskToSection(KanbanSectionSomeday)
			}
			return m, nil
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

	switch key {
	case "n":
		// Task new (create) - skip template picker
		var parentTask *models.Task
		if m.FocusedTask != nil {
			parentTask = m.FocusedTask
		} else {
			parentTask = m.getSelectedTask()
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: true}
		}
	case "N":
		// Task new (create) - show template picker if templates exist
		var parentTask *models.Task
		if m.FocusedTask != nil {
			parentTask = m.FocusedTask
		} else {
			parentTask = m.getSelectedTask()
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: false}
		}
	case "r":
		// Task rename (inline)
		if task != nil {
			return m, m.handleRenameKey()
		}
		return m, nil
	case "s":
		// Task section change
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			return m, func() tea.Msg {
				return ShowBulkSectionPickerMsg{Tasks: tasks, Project: project}
			}
		}
		if task != nil {
			return m, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: project}
			}
		}
		return m, nil
	case "o":
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return m, cmd
		}
		return m, nil
	case "p":
		// Task pin/unpin
		if m.multiSelectMode && m.selectedCount() > 0 {
			prevStates := captureTaskStates(m.Project.Tasks)
			count := m.selectedCount()
			for _, t := range m.getSelectedTasksList() {
				t.TogglePinned()
			}
			m.selectedTasks = make(map[string]bool)
			m.multiSelectMode = false
			m.statusMessage = fmt.Sprintf("Toggled pin on %d tasks", count)
			return m, m.saveWithWorkflow(prevStates, func() tea.Msg {
				return KanbanStatusChangedMsg{TaskID: "", PreviousSection: m.CurrentSection()}
			})
		}
		if task != nil {
			prevStates := captureTaskStates(m.Project.Tasks)
			task.TogglePinned()
			m.statusMessage = fmt.Sprintf("Pinned: %v", task.Pinned)
			return m, m.saveWithStatusChange(prevStates, task)
		}
		return m, nil
	case "t":
		// Task tags
		if m.multiSelectMode && m.selectedCount() > 0 {
			tasks := m.getSelectedTasksList()
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: project}
			}
		}
		if task != nil {
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
	case "c", "a":
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return m, cmd
		}
		return m, nil
	case "D":
		// Task delete
		if m.multiSelectMode && m.selectedCount() > 0 {
			m.statusMessage = fmt.Sprintf("Delete %d selected tasks? Press 'y' to confirm, 'n' to cancel", m.selectedCount())
			m.confirm.Show("delete", m.statusMessage)
			return m, nil
		}
		if task != nil {
			m.deleteTarget = task
			m.statusMessage = fmt.Sprintf("Delete task #%d? Press 'y' to confirm, 'n' to cancel", task.ShortID)
			m.confirm.Show("delete", m.statusMessage)
		}
		return m, nil
	case "m", "l", "d":
		if handled, cmd := handleTaskPrefixMsgKey(key, task, project); handled {
			return m, cmd
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
