package tui

import (
	"fmt"
	"strings"

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
		if m.confirm.IsActive() {
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
		if m.scoreFilterInput.IsActive() {
			cmd := m.scoreFilterInput.ProcessKey(msg, m.config, m.configManager, &m.filter.ScoreMin, m.applyFilterChange)
			return m, cmd
		}

		// Handle normal key input
		return m.handleKeyMsg(msg, visible)
	}

	return m, nil
}

// handleConfirmation handles confirmation prompts using the shared ConfirmDialog.
func (m *ProjectTreeModel) handleConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := m.confirm.Action()

	// Handle recurring task delete confirmation (d/s/n instead of y/n)
	if action == "delete-recurring" && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			m.confirm.Cancel()
			return m, m.deleteTask()
		case "s", "S":
			m.deleteTarget.ClearRecurrence()
			m.confirm.Cancel()
			return m, m.deleteTask()
		case "n", "N", "esc":
			m.confirm.Cancel()
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Standard y/n handling via ConfirmDialog
	confirmed, handled := m.confirm.HandleKey(key)
	if handled {
		if confirmed {
			switch action {
			case "archive":
				return m, m.archiveProject()
			case "unarchive":
				return m, m.unarchiveProject()
			case "delete":
				return m, m.deleteTask()
			case "bulk-delete":
				return m, m.bulkDeleteTasks()
			case "bulk-status":
				return m, m.executePendingBulkStatus()
			case "archive-tasks":
				return m, m.archiveCompletedTasks()
			}
		}
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
		if status, ok := StatusForPrefixKey(key); ok {
			_, cmd := m.handleStatusTransition(status, visible)
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
		// View mode switching (a, n, i, w, s, z, *, r, c)
		if vm, ok := viewModeForKey(key); ok {
			m.viewMode = vm
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		}

		// Go/section operations
		switch key {
		case "g":
			// Go to first task (Vim-style gg)
			m.cursor = 0
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
		if section, ok := sectionForMoveKey(key); ok {
			_, cmd := m.sendTasksToSection(section, visible)
			return m, cmd
		}
		switch key {
		case "K":
			// Move task to top of section
			return m.moveTaskToTop(visible)
		case "J":
			// Move task to bottom of section
			return m.moveTaskToBottom(visible)
		}

	case "!":
		if priority, ok := PriorityForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				prevStates := captureTaskStates(m.project.Tasks)
				count := applyBulkPriority(m.project, m.getSelectedTasks(), priority)
				m.clearSelection()
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d on %d tasks", priority, count))
				return m, m.saveAndRefreshWithPrevStates(prevStates)
			}
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				m.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", priority, task.Title))
				result := applyPriorityChange(m.workflowPersistFn(), m.project, task.ID, priority)
				if result.Err != nil {
					return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
				}
				m.buildTree()
				return m, result.Cmd
			}
			return m, nil
		}

	case "c":
		if !CursorInBounds(m.cursor, len(visible)) {
			return m, nil
		}
		if handled, cmd := handleLinkedTaskPrefixKey(key, visible[m.cursor].Task, m.project, true, true); handled {
			return m, cmd
		}
		return m, nil

	case "d":
		if days, isClear, ok := DueDateDaysForPrefixKey(key); ok {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				dueDate, desc := dueDateFromDays(days, isClear)
				prevStates := captureTaskStates(m.project.Tasks)
				count := applyBulkDueDate(m.project, m.getSelectedTasks(), dueDate)
				m.clearSelection()
				m.setUndoContext("Set Due Date", fmt.Sprintf("%s on %d tasks", desc, count))
				return m, m.saveAndRefreshWithPrevStates(prevStates)
			}
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				var result TaskOpResult
				if isClear {
					m.setUndoContext("Set Due Date", fmt.Sprintf("cleared: %s", task.Title))
					result = applyClearDueDate(m.workflowPersistFn(), m.project, task.ID)
				} else {
					m.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(days), task.Title))
					result = applyQuickDueDate(m.workflowPersistFn(), m.project, task.ID, days)
				}
				if result.Err != nil {
					return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
				}
				m.buildTree()
				return m, result.Cmd
			}
			return m, nil
		}
		if key == "d" {
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return SetDueDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

	case "F":
		// Filter prefix - first level
		return m.handleFilterPrefix(key)

	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS":
		return m.handleFilterSubPrefix(prefix, key)

	case "Fv":
		// Saved views sub-prefix
		return m.handleSavedViewPrefix(key)

	case "s":
		// Snooze quick actions
		if key == "d" {
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return SetSnoozeDateMsg{Task: task, Project: project}
				}
			}
			return m, nil
		}

		snoozeDate, desc, ok := resolveQuickSnoozeDate(key)
		if !ok {
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
	// Get cursor task (may be nil if cursor is out of bounds)
	var cursorTask *models.Task
	if CursorInBounds(m.cursor, len(visible)) {
		cursorTask = visible[m.cursor].Task
	}

	// Multi-select getter for shared handlers
	getMultiTasks := func() []*models.Task {
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m.getSelectedTasks()
		}
		return nil
	}

	// Try shared message-only handlers first (to, tc, ta, tl, td, tm)
	if cursorTask != nil {
		// tm is blocked for multi-select
		if key == "m" && m.multiSelectMode && len(m.selectedTasks) > 1 {
			return m, nil
		}
		if handled, cmd := handleTaskPrefixMsgKey(key, cursorTask, m.project); handled {
			return m, cmd
		}
	}

	// Try shared action handlers (tn, tN, ts, tt)
	if handled, cmd := handleTaskActionPrefixKey(key, cursorTask, m.project, m.focusedTask, getMultiTasks); handled {
		return m, cmd
	}

	switch key {
	case "r":
		if cursorTask != nil {
			m.enterQuickEditMode(cursorTask, true)
		}
		return m, nil
	case "p":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			count := len(m.selectedTasks)
			for _, task := range m.getSelectedTasks() {
				task.TogglePinned()
			}
			m.clearSelection()
			m.setUndoContext("Toggle Pin", fmt.Sprintf("%d tasks", count))
			return m, m.saveAndRefresh()
		}
		if cursorTask != nil {
			m.setUndoContext("Toggle Pin", cursorTask.Title)
			result := applyTogglePinned(m.workflowPersistFn(), m.project, cursorTask.ID)
			if result.Err != nil {
				return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
			}
			m.buildTree()
			return m, result.Cmd
		}
		return m, nil
	case "D":
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			m.showBulkDeleteConfirm()
			return m, nil
		}
		if cursorTask != nil {
			m.showDeleteConfirm(cursorTask)
		}
		return m, nil
	}
	return m, nil
}

// handleBlockerPrefix handles the second key after pressing b (blocker/dependency prefix).
func (m *ProjectTreeModel) handleBlockerPrefix(key string, visible []*TreeNode) (tea.Model, tea.Cmd) {
	if !CursorInBounds(m.cursor, len(visible)) {
		return m, nil
	}
	if handled, cmd := handleBlockerPrefixKey(key, visible[m.cursor].Task, m.project); handled {
		return m, cmd
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

	// Sidebar keys have priority when sidebar is focused
	if m.sidebar.IsFocused() {
		_, cmd := m.sidebar.HandleKeys(m, msg)
		// When sidebar is focused, consume all keys to prevent tree navigation
		return m, cmd
	}

	// Prefix key activation (shared across primary views)
	if prefix, ok := activatePrefixKey(key, CursorInBounds(m.cursor, len(visible))); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Special case for search activation
	if key == "/" && !m.confirm.IsActive() {
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
			m.confirm.Show("unarchive", "Are you sure you want to unarchive this project? (y/n)")
		} else {
			m.confirm.Show("archive", "Are you sure you want to archive this project? (y/n)")
		}
		return m, nil
	}

	// Shared primary view keys (view switching, project cycling, secondary views, capture, etc.)
	if cmd, handled := handleCommonPrimaryViewKey(key, m); handled {
		return m, cmd
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
