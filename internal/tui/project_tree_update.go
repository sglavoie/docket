package tui

import (
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
			cmd := m.scoreFilterInput.ProcessKey(msg, m.runtime, &m.filter.ScoreMin, m.applyFilterChange)
			return m, cmd
		}

		// Handle normal key input
		return m.handleKeyMsg(msg, visible)
	}

	return m, nil
}

// handleKeyMsg dispatches key messages to appropriate handlers.
func (m *ProjectTreeModel) handleKeyMsg(msg tea.KeyPressMsg, _ []*TreeNode) (tea.Model, tea.Cmd) {
	// Sidebar keys have priority when sidebar is focused
	if m.sidebar.IsFocused() {
		_, cmd := m.sidebar.HandleKeys(m, msg)
		return m, cmd
	}

	// Dispatcher handles all remaining keys
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		if model != nil {
			return model, cmd
		}
		return m, cmd
	}
	return m, nil
}
