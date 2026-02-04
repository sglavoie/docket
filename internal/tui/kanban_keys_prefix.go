package tui

import (
	"fmt"
	"time"

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
				m.Project.MutateTask(task.ID, func(t *models.Task) {
					t.Priority = priority
				})
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
		m.Project.MutateTask(task.ID, func(t *models.Task) {
			t.DueDate = dueDate
		})
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

	case "F":
		// Filter prefix - first level
		return m.handleKanbanFilterPrefix(key)

	case "Ft":
		// Filter type sub-prefix
		return m.handleKanbanFilterTypePrefix(key)

	case "Fs":
		// Filter status sub-prefix
		return m.handleKanbanFilterStatusPrefix(key)

	case "Fd":
		// Filter due-date sub-prefix
		return m.handleKanbanFilterDuePrefix(key)

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
		// Task open in external editor
		if task != nil {
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: project}
			}
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
	case "c":
		// Task comments
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskCommentsMsg{Task: task, Project: project}
			}
		}
		return m, nil
	case "a":
		// Task attachments
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskAttachmentsMsg{Task: task, Project: project}
			}
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
	case "m":
		// Task move (to another project)
		if task != nil {
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
			}
		}
		return m, nil
	case "l":
		// Task checklist
		if task != nil {
			return m, func() tea.Msg {
				return ShowTaskChecklistMsg{Task: task, Project: project}
			}
		}
		return m, nil
	case "d":
		// Task description editor (text popup)
		if task != nil {
			return m, func() tea.Msg {
				return ShowDescriptionEditorMsg{Task: task, Project: project}
			}
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
	project := m.Project

	switch key {
	case "b":
		// Blocks/blockers (view dependencies)
		return m, func() tea.Msg {
			return ShowDependencyViewerMsg{Task: task, Project: project}
		}
	case "m":
		// Blocks managed (dependency management)
		return m, func() tea.Msg {
			return ManageDependenciesMsg{Task: task, Project: project}
		}
	}
	return m, nil
}
