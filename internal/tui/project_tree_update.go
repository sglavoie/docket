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
		m.undo.TaskIDs = []string{task.ID}
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

// handleKeyMsg dispatches key messages to appropriate handlers.
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyPressMsg, visible []*TreeNode) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys first (which-key style two-key sequences)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
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

	// Export project to clean Markdown.
	if key == "ctrl+e" {
		project := m.getProject()
		return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
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
