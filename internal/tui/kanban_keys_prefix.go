package tui

import (
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
			if m.multiSelectMode && m.SelectedCount() > 0 {
				return m, m.handleBulkStatusChange(status)
			}
			return m, m.handleStatusTransition(status)
		}

	case "!":
		if cmd, ok := handlePriorityPrefix(key, m); ok {
			return m, cmd
		}

	case "d":
		if cmd, ok := handleDueDatePrefix(key, m); ok {
			return m, cmd
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
		if model, cmd, handled := handleSectionMovePrefix(key, SectionMoveCallbacks{
			MoveToSection: func(section string) (tea.Model, tea.Cmd) {
				return m, m.sendTaskToSection(section)
			},
			BulkMoveToSection: func(section string) (tea.Model, tea.Cmd) {
				return m, m.handleBulkSectionMove(section)
			},
			MoveToTop:      func() (tea.Model, tea.Cmd) { return m, m.moveTaskToTop() },
			MoveToBottom:   func() (tea.Model, tea.Cmd) { return m, m.moveTaskToBottom() },
			IsMultiSelect:  func() bool { return m.multiSelectMode && m.SelectedCount() > 0 },
			CanMoveSection: func() bool { return IsSectionGroupBy(m.groupBy) },
		}); handled {
			return model, cmd
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
	cmd, handled := handleSharedTaskPrefix(key, m, m.FocusedTask, func(task *models.Task) tea.Cmd {
		return m.quickEdit.Enter(task, true)
	})
	if handled {
		return m, cmd
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
