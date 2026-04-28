package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectTreeModel) handlePrefixKey(key string, visible []*TreeNode) (tea.Model, tea.Cmd) { //nolint:gocyclo
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	if key == "esc" {
		return m, nil
	}

	switch prefix {
	case ",":
		if key == "k" {
			cmd, _ := handleSkipRecurrence(m)
			return m, cmd
		}
		if key == "D" {
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
		switch key {
		case "a":
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
			return m, nil
		case "o":
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = true
				m.updateVisibility()
			}
			return m, nil
		case "c":
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				node.Expanded = false
				m.updateVisibility()
			}
			return m, nil
		case "R":
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
			return m, nil
		case "M":
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
			return m, nil
		case "v":
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
		if vm, ok := viewModeForKey(key); ok {
			m.viewMode = vm
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		}

		switch key {
		case "g":
			old := m.cursor
			m.cursor = 0
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		case "o":
			if CursorInBounds(m.cursor, len(visible)) {
				node := visible[m.cursor]
				if node.IsContextNode {
					task := node.Task
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
			m.filter.DueDate = (m.filter.DueDate + 1) % 5
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "v":
			m.viewMode = (m.viewMode + 1) % (ViewModeAll + 1)
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		case "X":
			if m.multiSelectMode {
				for _, node := range m.visibleNodes() {
					delete(m.selectedTasks, node.Task.ID)
				}
			}
			return m, nil
		case "h":
			if CursorInBounds(m.cursor, len(visible)) {
				task := visible[m.cursor].Task
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: task, Project: project}
				}
			}
			return m, nil
		case "l":
			project := m.project
			return m, func() tea.Msg {
				return ShowProjectRecentActivityMsg{Project: project}
			}
		case "k":
			project := m.project
			return m, func() tea.Msg {
				return ShowKanbanMsg{Project: project}
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
		return m.handleFilterPrefix(key)

	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS", "F.":
		return m.handleFilterSubPrefix(prefix, key)

	case "Fv":
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
		return m.handleTaskPrefix(key, visible)

	case "b":
		return m.handleBlockerPrefix(key, visible)
	}

	return m, nil
}

// handleTaskPrefix handles the second key after pressing t (task action prefix).
func (m *ProjectTreeModel) handleTaskPrefix(key string, _ []*TreeNode) (tea.Model, tea.Cmd) {
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
