package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *ProjectTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleNodes()

	switch msg := msg.(type) {
	case treeRefreshMsg:
		m.buildTree()
		m.clampCursor()
		return m, nil

	case ParentSelectedMsg:
		if msg.ReparentTaskID == "" {
			return m, nil
		}
		task := m.project.GetTaskByID(msg.ReparentTaskID)
		if task == nil {
			return m, nil
		}
		prevStates := captureTaskStates(m.project.Tasks)
		if msg.ParentID == nil {
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.ParentID = nil
			})
		} else {
			targetID := *msg.ParentID
			taskMap := make(map[string]*models.Task, len(m.project.Tasks))
			for _, t := range m.project.Tasks {
				taskMap[t.ID] = t
			}
			targetParent := taskMap[targetID]
			if targetParent != nil && task.Type.CanBeChildOf(targetParent.Type) && !models.WouldCreateCycle(taskMap, task.ID, targetID) {
				m.project.MutateTask(task.ID, func(t *models.Task) {
					t.ParentID = &targetID
				})
			}
		}
		m.project.BuildTaskTree()
		m.setUndoContext("Reparent Task", task.Title)
		return m, m.saveAndRefreshBulk(prevStates)

	case tea.KeyPressMsg:
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
func (m *ProjectTreeModel) handleConfirmation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	if confirmed, handled := m.HandleConfirmKey(key); handled {
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
func (m *ProjectTreeModel) handleSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.updateVisibility()
		visible := m.visibleNodes()
		m.cursor = ClampCursor(m.cursor, len(visible))
	})
	if handled {
		return model, cmd
	}
	return m, cmd
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *ProjectTreeModel) handleQuickEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	_, cmd := handleQuickEditKey(msg, m.quickEdit, m.project, m.saveAndRefresh, m.setUndoContext)
	return m, cmd
}

// enterQuickEditMode starts inline editing for the current task.
func (m *ProjectTreeModel) enterQuickEditMode(task *models.Task, renameOnly bool) {
	m.quickEdit.Enter(task, renameOnly)
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectTreeModel) handlePrefixKey(key string, visible []*TreeNode) (tea.Model, tea.Cmd) { //nolint:gocyclo
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	switch prefix {
	case ",":
		if key == "k" {
			cmd, _ := handleSkipRecurrence(m)
			return m, cmd
		}
		if key == "D" {
			// Enter defer sub-prefix
			m.pendingPrefix = ",D"
			return m, nil
		}
		if status, ok := StatusForPrefixKey(key); ok {
			_, cmd := m.handleStatusTransition(status, visible)
			return m, cmd
		}

	case ",D":
		if cmd, ok := handleDeferRecurrence(key, m); ok {
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
		case "p":
			m.previewMode = !m.previewMode
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
			old := m.cursor
			m.cursor = 0
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		case "o":
			// Jump to context node's home section
			if CursorInBounds(m.cursor, len(visible)) {
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
						case models.SectionStandby:
							m.viewMode = ViewModeStandby
						default:
							m.viewMode = ViewModeActive
						}
					}
					m.buildTree()
					return m, m.NavigateToTask(task)
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
		case "l":
			// Go to project recent activity
			project := m.project
			return m, func() tea.Msg {
				return ShowProjectRecentActivityMsg{Project: project}
			}
		}

	case "m":
		if model, cmd, handled := handleSectionMovePrefix(key, SectionMoveCallbacks{
			MoveToSection: func(section string) (tea.Model, tea.Cmd) {
				_, cmd := m.sendTasksToSection(section, visible)
				return m, cmd
			},
			BulkMoveToSection: func(section string) (tea.Model, tea.Cmd) {
				_, cmd := m.sendTasksToSection(section, visible)
				return m, cmd
			},
			MoveToTop:     func() (tea.Model, tea.Cmd) { return m.moveTaskToTop(visible) },
			MoveToBottom:  func() (tea.Model, tea.Cmd) { return m.moveTaskToBottom(visible) },
			IsMultiSelect: func() bool { return m.multiSelectMode && len(m.selectedTasks) > 0 },
		}); handled {
			return model, cmd
		}

	case "!":
		if cmd, ok := handlePriorityPrefix(key, m); ok {
			return m, cmd
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
		if cmd, ok := handleDueDatePrefix(key, m); ok {
			return m, cmd
		}

	case "F":
		// Filter prefix - first level
		return m.handleFilterPrefix(key)

	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS", "F.":
		return m.handleFilterSubPrefix(prefix, key)

	case "Fv":
		// Saved filters sub-prefix
		return m.handleSavedFilterPrefix(key)

	case ".":
		if cmd, ok := handleStagePrefix(key, m, m.config.Stages); ok {
			return m, cmd
		}

	case "s":
		if cmd, ok := handleSnoozePrefix(key, m); ok {
			return m, cmd
		}

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
func (m *ProjectTreeModel) handleTaskPrefix(key string, _ []*TreeNode) (tea.Model, tea.Cmd) {
	// tm is blocked for multi-select in ProjectTree
	if key == "m" && m.multiSelectMode && len(m.selectedTasks) > 1 {
		return m, nil
	}

	cmd, handled := handleSharedTaskPrefix(key, m, m.focusedTask, func(task *models.Task) tea.Cmd {
		m.enterQuickEditMode(task, true)
		return nil
	})
	if handled {
		return m, cmd
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
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyPressMsg, visible []*TreeNode) (tea.Model, tea.Cmd) {
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
	cursorOnTask := CursorInBounds(m.cursor, len(visible))
	if prefix, ok := activatePrefixKey(key, cursorOnTask); ok {
		m.pendingPrefix = prefix
		return m, nil
	}
	if isPrefixKeySkippedOnHeader(key, cursorOnTask) {
		m.setStatus("Move cursor to a task first")
		return m, nil
	}

	// Special case for search activation
	if key == "/" && !m.confirm.IsActive() {
		return m, m.ActivateSearch()
	}

	// Esc cascade: dismiss the first active layer
	if key == "esc" {
		RunEscCascade(m.escSteps())
		return m, nil
	}

	// Special case for ctrl+a (archive project)
	if key == "ctrl+a" {
		if m.project.Sandbox {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
			}
		}
		if m.project.Archived {
			m.confirm.Show("unarchive", "Are you sure you want to unarchive this project?")
		} else {
			m.confirm.Show("archive", "Are you sure you want to archive this project?")
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
	if handled, cmd := m.handleTypeKeys(key); handled {
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
