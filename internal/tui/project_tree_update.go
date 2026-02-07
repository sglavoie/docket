package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case treeRefreshMsg:
		m.buildTree()
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		// Handle confirmation prompts first
		if m.confirmAction != ConfirmNone {
			return m.handleConfirmation(msg)
		}

		// Handle search mode
		if m.IsSearchMode() {
			return m.handleSearchMode(msg)
		}

		// Handle quick edit mode for inline task editing
		if m.quickEdit.IsActive() {
			return m.handleQuickEditMode(msg)
		}

		// Handle score filter custom value input
		if m.scoreFilterInputMode {
			return m.handleScoreFilterInput(msg)
		}

		// Handle normal key input
		return m.handleKeyMsg(msg, visible)
	}

	return m, nil
}

// handleConfirmation handles y/n confirmation prompts.
func (m *ProjectTreeModel) handleConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle recurring task delete confirmation (d/s/n)
	if m.confirmAction == ConfirmDeleteRecurring && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			// Delete instance only - keeps recurrence, next instance will still generate
			m.confirmAction = ConfirmNone
			return m, m.deleteTask()
		case "s", "S":
			// Stop recurrence - clear recurrence pattern then delete
			m.deleteTarget.ClearRecurrence()
			m.confirmAction = ConfirmNone
			return m, m.deleteTask()
		case "n", "N", "esc":
			m.confirmAction = ConfirmNone
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Handle standard y/n confirmations
	switch key {
	case "y", "Y":
		switch m.confirmAction {
		case ConfirmArchive:
			m.confirmAction = ConfirmNone
			return m, m.archiveProject()
		case ConfirmUnarchive:
			m.confirmAction = ConfirmNone
			return m, m.unarchiveProject()
		case ConfirmDelete:
			m.confirmAction = ConfirmNone
			return m, m.deleteTask()
		case ConfirmBulkDelete:
			m.confirmAction = ConfirmNone
			return m, m.bulkDeleteTasks()
		case ConfirmBulkStatus:
			m.confirmAction = ConfirmNone
			return m, m.executePendingBulkStatus()
		case ConfirmArchiveTasks:
			m.confirmAction = ConfirmNone
			return m, m.archiveCompletedTasks()
		}
	case "n", "N", "esc":
		m.confirmAction = ConfirmNone
		m.deleteTarget = nil
		m.clearPendingBulkStatus()
		return m, nil
	}
	return m, nil
}

// handleSearchMode handles input while in search mode.
func (m *ProjectTreeModel) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handled, cmd, valueChanged := m.HandleSearchInput(msg)
	if valueChanged {
		m.updateVisibility()
		visible := m.visibleNodes()
		m.cursor = ClampCursor(m.cursor, len(visible))
	}
	if handled {
		return m, cmd
	}
	return m, cmd
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *ProjectTreeModel) handleQuickEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.exitQuickEditMode()
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.quickEdit.GetInput().Value())
		if input == "" {
			m.exitQuickEditMode()
			return m, nil
		}

		// Re-lookup task to avoid stale pointer after concurrent reload
		task := m.project.GetTaskByID(m.quickEdit.GetTask().ID)
		if task == nil {
			m.exitQuickEditMode()
			return m, nil
		}

		// Parse and apply the edit
		if m.quickEdit.IsRenameOnly() {
			// Rename-only mode: just update title, no modifier parsing
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.Title = input
			})
			m.setUndoContext("Rename Task", task.Title)
		} else {
			// Full quick edit mode: parse modifiers
			result, err := parseQuickEdit(input, task, m.project)
			if err != nil {
				// Keep edit mode open with error displayed
				return m, nil
			}
			applyQuickEdit(task, m.project, result)
			m.setUndoContext("Quick Edit", task.Title)
		}

		m.exitQuickEditMode()
		return m, m.saveAndRefresh()

	default:
		// Update the text input
		var cmd tea.Cmd
		input := m.quickEdit.GetInput()
		input, cmd = input.Update(msg)
		m.quickEdit.UpdateInput(input)

		// Update preview (only in full edit mode, not rename mode)
		if !m.quickEdit.IsRenameOnly() {
			inputValue := m.quickEdit.GetInput().Value()
			preview, _ := parseQuickEditForPreview(inputValue, m.quickEdit.GetTask(), m.project)
			m.quickEdit.SetPreview(preview)
		}

		return m, cmd
	}
}

// enterQuickEditMode starts inline editing for the current task.
func (m *ProjectTreeModel) enterQuickEditMode(task *models.Task, renameOnly bool) {
	m.quickEdit.Enter(task, renameOnly)
}

// exitQuickEditMode cleans up quick edit state.
func (m *ProjectTreeModel) exitQuickEditMode() {
	m.quickEdit.Exit()
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectTreeModel) handlePrefixKey(key string, visible []*TreeNode) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		// Status operations
		switch key {
		case "s":
			_, cmd := m.handleStartTask(visible)
			return m, cmd
		case "d":
			_, cmd := m.handleCompleteTask(visible)
			return m, cmd
		case "b":
			_, cmd := m.handleBlockTask(visible)
			return m, cmd
		case "x":
			_, cmd := m.handleCancelTask(visible)
			return m, cmd
		case "o":
			_, cmd := m.handleReopenTask(visible)
			return m, cmd
		case "r":
			_, cmd := m.handleReviewTask(visible)
			return m, cmd
		}

	case "z":
		// Fold operations
		switch key {
		case "a":
			// Toggle expand/collapse current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
			return m, nil
		case "o":
			// Expand current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = true
				m.updateVisibility()
			}
			return m, nil
		case "c":
			// Collapse current node
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = false
				m.updateVisibility()
			}
			return m, nil
		case "R":
			// Expand all
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
			return m, nil
		case "M":
			// Collapse all
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
			return m, nil
		case "v":
			// Focus mode (zoom into subtree)
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				if task.Type.CanHaveChildren() {
					m.focusedTask = task
					m.cursor = 0
					m.buildTree()
				}
			}
			return m, nil
		}

	case "g":
		// Go/section operations
		switch key {
		case "g":
			// Go to first task (Vim-style gg)
			m.cursor = 0
			return m, nil
		case "a":
			// Go to Active view
			m.viewMode = ViewModeActive
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "n":
			// Go to Next view
			m.viewMode = ViewModeNext
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "i":
			// Go to Inbox view
			m.viewMode = ViewModeInbox
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "w":
			// Go to Waiting view
			m.viewMode = ViewModeWaiting
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "s":
			// Go to Someday view
			m.viewMode = ViewModeSomeday
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "z":
			// Go to Snoozed view
			m.viewMode = ViewModeSnoozed
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "*":
			// Go to All view
			m.viewMode = ViewModeAll
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "r":
			// Go to Review view
			m.viewMode = ViewModeReview
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "c":
			// Go to Completed view
			m.viewMode = ViewModeCompleted
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "o":
			// Jump to context node's home section
			if m.cursor < len(visible) {
				node := visible[m.cursor]
				if node.IsContextNode {
					task := node.Task
					// Switch to the task's home section first so NavigateToTask
					// finds it as intrinsically visible rather than a context node.
					if task.IsComplete() {
						m.viewMode = ViewModeCompleted
					} else {
						switch task.Section {
						case models.SectionNext:
							m.viewMode = ViewModeNext
						case models.SectionInbox:
							m.viewMode = ViewModeInbox
						case models.SectionWaiting:
							m.viewMode = ViewModeWaiting
						case models.SectionSomeday:
							m.viewMode = ViewModeSomeday
						case models.SectionReview:
							m.viewMode = ViewModeReview
						default:
							m.viewMode = ViewModeActive
						}
					}
					m.buildTree()
					m.NavigateToTask(task)
					return m, nil
				}
			}
			return m, nil
		case "d":
			// Cycle due date filter
			m.filter.DueDate = (m.filter.DueDate + 1) % 5
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "v":
			// Cycle view modes (including Completed, Snoozed, and All)
			m.viewMode = (m.viewMode + 1) % 9
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "X":
			// Deselect all (in multi-select mode)
			if m.multiSelectMode {
				for _, node := range m.visibleNodes() {
					delete(m.selectedTasks, node.Task.ID)
				}
			}
			return m, nil
		case "h":
			// Go to task activity history
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "m":
		// Move/send-to-section operations
		switch key {
		case "a":
			// Send to Active section
			_, cmd := m.sendTasksToActive(visible)
			return m, cmd
		case "n":
			// Send to Next section
			_, cmd := m.sendTasksToSection(models.SectionNext, visible)
			return m, cmd
		case "i":
			// Send to Inbox section
			_, cmd := m.sendTasksToSection(models.SectionInbox, visible)
			return m, cmd
		case "w":
			// Send to Waiting section
			_, cmd := m.sendTasksToSection(models.SectionWaiting, visible)
			return m, cmd
		case "r":
			// Send to Review section
			_, cmd := m.sendTasksToSection(models.SectionReview, visible)
			return m, cmd
		case "s":
			// Send to Someday section
			_, cmd := m.sendTasksToSection(models.SectionSomeday, visible)
			return m, cmd
		case "K":
			// Move task to top of section
			return m.moveTaskToTop(visible)
		case "J":
			// Move task to bottom of section
			return m.moveTaskToBottom(visible)
		}

	case "!":
		// Priority quick actions
		switch key {
		case "1", "2", "3", "4", "5":
			priority := int(key[0] - '0')
			// Capture task states before changes for event-based workflow triggers
			prevStates := captureTaskStates(m.project.Tasks)
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				count := 0
				for id := range m.selectedTasks {
					if m.project.MutateTask(id, func(t *models.Task) {
						t.Priority = priority
					}) {
						count++
					}
				}
				m.clearSelection()
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d on %d tasks", priority, count))
				return m, m.saveAndRefreshWithPrevStates(prevStates)
			}
			if CursorInBounds(m.cursor, len(visible)) {
				task := m.project.GetTaskByID(visible[m.cursor].Task.ID)
				if task == nil {
					return m, nil
				}
				m.project.MutateTask(task.ID, func(t *models.Task) {
					t.Priority = priority
				})
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, task.Title))
				return m, m.saveAndRefreshWithPrevStates(prevStates)
			}
			return m, nil
		}

	case "c":
		// Create linked task or navigate
		if !CursorInBounds(m.cursor, len(visible)) {
			return m, nil
		}
		task := visible[m.cursor].Task
		project := m.project

		// Navigate to linked tasks
		if key == "n" {
			return m, func() tea.Msg {
				return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
			}
		}

		// Unlink linked task (opens selector where user can press x to unlink)
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

	case "d":
		// Due date quick actions
		now := time.Now()
		var dueDate *time.Time
		var desc string

		switch key {
		case "t":
			// Today
			today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
			dueDate = &today
			desc = "today"
		case "m":
			// Tomorrow
			tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 0, now.Location())
			dueDate = &tomorrow
			desc = "tomorrow"
		case "w":
			// Next week (+7 days)
			nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 0, now.Location())
			dueDate = &nextWeek
			desc = "+7 days"
		case "n":
			// Next month (+30 days)
			nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 23, 59, 59, 0, now.Location())
			dueDate = &nextMonth
			desc = "+30 days"
		case "0":
			// Clear due date
			dueDate = nil
			desc = "cleared"
		case "d":
			// Open full date picker
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		default:
			return m, nil
		}

		// Capture task states before changes for event-based workflow triggers
		prevStates := captureTaskStates(m.project.Tasks)

		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := 0
			for id := range m.selectedTasks {
				if m.project.MutateTask(id, func(t *models.Task) {
					t.DueDate = dueDate
				}) {
					count++
				}
			}
			m.clearSelection()
			m.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
			return m, m.saveAndRefreshWithPrevStates(prevStates)
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := m.project.GetTaskByID(visible[m.cursor].Task.ID)
			if task == nil {
				return m, nil
			}
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.DueDate = dueDate
			})
			m.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", desc, task.Title))
			return m, m.saveAndRefreshWithPrevStates(prevStates)
		}
		return m, nil

	case "F":
		// Filter prefix - first level
		return m.handleFilterPrefix(key)

	case "Ft":
		// Filter type sub-prefix
		return m.handleFilterTypePrefix(key)

	case "Fs":
		// Filter status sub-prefix
		return m.handleFilterStatusPrefix(key)

	case "Fd":
		// Filter due-date sub-prefix
		return m.handleFilterDuePrefix(key)

	case "Fa":
		// Filter attribute sub-prefix
		return m.handleFilterAttrPrefix(key)

	case "Fas":
		// Filter attribute severity value
		return m.handleFilterAttrSeverityPrefix(key)

	case "Far":
		// Filter attribute reproducible value
		return m.handleFilterAttrReproduciblePrefix(key)

	case "Fae":
		// Filter attribute effort value
		return m.handleFilterAttrEffortPrefix(key)

	case "Fac":
		// Filter attribute category value
		return m.handleFilterAttrCategoryPrefix(key)

	case "Fav":
		// Filter attribute value value
		return m.handleFilterAttrValuePrefix(key)

	case "Faw":
		// Filter attribute worktype value
		return m.handleFilterAttrWorktypePrefix(key)

	case "Fao":
		// Filter attribute scope value
		return m.handleFilterAttrScopePrefix(key)

	case "Fap":
		// Filter attribute points value
		return m.handleFilterAttrPointsPrefix(key)

	case "FS":
		// Filter score threshold
		return m.handleFilterScorePrefix(key)

	case "Fv":
		// Saved views sub-prefix
		return m.handleSavedViewPrefix(key)

	case "s":
		// Snooze quick actions
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		var snoozeDate *time.Time
		var desc string

		switch key {
		case "t":
			// Tomorrow
			d := today.AddDate(0, 0, 1)
			snoozeDate = &d
			desc = "tomorrow"
		case "w":
			// 1 week
			d := today.AddDate(0, 0, 7)
			snoozeDate = &d
			desc = "+1 week"
		case "m":
			// 1 month
			d := today.AddDate(0, 1, 0)
			snoozeDate = &d
			desc = "+1 month"
		case "n":
			// Next Monday
			daysUntilMon := (8 - int(today.Weekday())) % 7
			if daysUntilMon == 0 {
				daysUntilMon = 7
			}
			d := today.AddDate(0, 0, daysUntilMon)
			snoozeDate = &d
			desc = "next Monday"
		case "0":
			// Clear snooze
			snoozeDate = nil
			desc = "cleared"
		case "d":
			// Open date picker for snooze
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return SetSnoozeDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		default:
			return m, nil
		}

		// Capture task states before changes for event-based workflow triggers
		prevStates := captureTaskStates(m.project.Tasks)

		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := 0
			for id := range m.selectedTasks {
				if m.project.MutateTask(id, func(t *models.Task) {
					t.SnoozedUntil = snoozeDate
				}) {
					count++
				}
			}
			m.clearSelection()
			m.setUndoContext("Snooze", fmt.Sprintf("%s on %d tasks", desc, count))
			return m, m.saveAndRefreshWithPrevStates(prevStates)
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := m.project.GetTaskByID(visible[m.cursor].Task.ID)
			if task == nil {
				return m, nil
			}
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.SnoozedUntil = snoozeDate
			})
			m.setUndoContext("Snooze", fmt.Sprintf("%s: %s", desc, task.Title))
			return m, m.saveAndRefreshWithPrevStates(prevStates)
		}
		return m, nil

	case "t":
		// Task action prefix
		return m.handleTaskPrefix(key, visible)

	case "b":
		// Blocker/dependency prefix
		return m.handleBlockerPrefix(key, visible)
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}

// handleTaskPrefix handles the second key after pressing t (task action prefix).
func (m *ProjectTreeModel) handleTaskPrefix(key string, visible []*TreeNode) (tea.Model, tea.Cmd) {
	switch key {
	case "n":
		// Task new (create) - skip template picker
		var parentTask *models.Task
		if m.focusedTask != nil {
			parentTask = m.focusedTask
		} else if CursorInBounds(m.cursor, len(visible)) {
			parentTask = visible[m.cursor].Task
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: true}
		}
	case "N":
		// Task new (create) - show template picker if templates exist
		var parentTask *models.Task
		if m.focusedTask != nil {
			parentTask = m.focusedTask
		} else if CursorInBounds(m.cursor, len(visible)) {
			parentTask = visible[m.cursor].Task
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{ParentTask: parentTask, SkipTemplatePicker: false}
		}
	case "r":
		// Task rename (inline)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			m.enterQuickEditMode(task, true)
		}
		return m, nil
	case "s":
		// Task section change
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			project := m.project
			return m, func() tea.Msg {
				return ShowBulkSectionPickerMsg{Tasks: tasks, Project: project}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "o":
		// Task open in external editor
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return EditTaskContentMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "p":
		// Task pin/unpin
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.TogglePinned()
			}
			m.clearSelection()
			m.setUndoContext("Toggle Pin", fmt.Sprintf("%d tasks", count))
			return m, m.saveAndRefresh()
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			task.TogglePinned()
			m.setUndoContext("Toggle Pin", task.Title)
			return m, m.saveAndRefresh()
		}
		return m, nil
	case "t":
		// Task tags
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			tasks := m.getSelectedTasks()
			return m, func() tea.Msg {
				return ShowBulkTagSelectorMsg{Tasks: tasks, Project: m.project}
			}
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{
					Task:        task,
					Project:     m.project,
					CurrentTags: append([]string{}, task.Tags...),
				}
			}
		}
		return m, nil
	case "c":
		// Task comments
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowTaskCommentsMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "a":
		// Task attachments
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowTaskAttachmentsMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "D":
		// Task delete
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.confirmAction = ConfirmBulkDelete
			return m, nil
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
		return m, nil
	case "m":
		// Task move (reparent)
		if m.multiSelectMode && len(m.selectedTasks) > 1 {
			return m, nil
		}
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{
					Task:          task,
					SourceProject: m.project,
				}
			}
		}
		return m, nil
	case "l":
		// Task checklist
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowTaskChecklistMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "d":
		// Task description editor (text popup)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowDescriptionEditorMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	}
	return m, nil
}

// handleBlockerPrefix handles the second key after pressing b (blocker/dependency prefix).
func (m *ProjectTreeModel) handleBlockerPrefix(key string, visible []*TreeNode) (tea.Model, tea.Cmd) {
	switch key {
	case "b":
		// Blocks/blockers (view dependencies)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ShowDependencyViewerMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	case "m":
		// Blocks managed (dependency management)
		if CursorInBounds(m.cursor, len(visible)) {
			task := visible[m.cursor].Task
			return m, func() tea.Msg {
				return ManageDependenciesMsg{Task: task, Project: m.project}
			}
		}
		return m, nil
	}
	return m, nil
}

// handleKeyMsg dispatches key messages to appropriate handlers.
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyMsg, visible []*TreeNode) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key, visible)
	}

	// Get selected task for sidebar navigation
	var selectedTask *models.Task
	if len(visible) > 0 && m.cursor >= 0 && m.cursor < len(visible) {
		selectedTask = visible[m.cursor].Task
	}

	// Sidebar keys have priority when sidebar is focused
	if m.sidebar.IsFocused() {
		if handled, cmd := m.handleSidebarKeys(key, selectedTask); handled {
			return m, cmd
		}
	}

	// Prefix key activation (status, fold, go/section, priority, due date, create linked, task, blocker)
	switch key {
	case ",":
		m.pendingPrefix = ","
		return m, nil
	case "z":
		m.pendingPrefix = "z"
		return m, nil
	case "g":
		m.pendingPrefix = "g"
		return m, nil
	case "m":
		m.pendingPrefix = "m"
		return m, nil
	case "!":
		m.pendingPrefix = "!"
		return m, nil
	case "d":
		m.pendingPrefix = "d"
		return m, nil
	case "s":
		m.pendingPrefix = "s"
		return m, nil
	case "c":
		if CursorInBounds(m.cursor, len(visible)) {
			m.pendingPrefix = "c"
			return m, nil
		}
		return m, nil
	case "F":
		m.pendingPrefix = "F"
		return m, nil
	case "t":
		// Task action prefix
		m.pendingPrefix = "t"
		return m, nil
	case "b":
		// Blocker/dependency prefix
		m.pendingPrefix = "b"
		return m, nil
	}

	// Special case for search activation
	if key == "/" && m.confirmAction == ConfirmNone {
		return m, m.ActivateSearch()
	}

	// Special case for esc (exit multi-select, focus mode, or clear search)
	if key == "esc" {
		if m.multiSelectMode {
			m.clearSelection()
			return m, nil
		}
		if m.focusedTask != nil {
			m.focusedTask = nil
			m.buildTree()
			m.clampCursor()
			return m, nil
		}
		if m.SearchQuery() != "" {
			m.ClearSearch()
			m.updateVisibility()
			return m, nil
		}
	}

	// Special case for ctrl+a (archive project)
	if key == "ctrl+a" {
		if m.project.Sandbox {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
			}
		}
		if m.project.Archived {
			m.confirmAction = ConfirmUnarchive
		} else {
			m.confirmAction = ConfirmArchive
		}
		return m, nil
	}

	// Project cycling keys (emit messages to app level)
	switch key {
	case "`":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: true} }
	case "~":
		return m, func() tea.Msg { return CycleProjectMsg{Forward: false} }
	case "@":
		return m, func() tea.Msg { return ShowProjectCycleManagerMsg{} }
	}

	// Context filter (capital C, doesn't conflict with lowercase c prefix)
	if key == "C" {
		return m, func() tea.Msg {
			return ShowContextPickerMsg{}
		}
	}

	// Try each handler group
	if handled, cmd := m.handleNavigationKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleTypeKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleMovementKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSelectionKeys(key, visible); handled {
		return m, cmd
	}
	if handled, cmd := m.handleFilterBarKeys(key); handled {
		return m, cmd
	}
	if handled, cmd := m.handleFilterKeys(key); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSortKeys(key); handled {
		return m, cmd
	}
	if handled, cmd := m.handleActionKeys(key, visible); handled {
		return m, cmd
	}

	return m, nil
}
