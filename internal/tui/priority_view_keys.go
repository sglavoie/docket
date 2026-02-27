package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

func (m *PriorityViewModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle search input mode
	if m.IsSearchMode() {
		handled, cmd, valueChanged := m.HandleSearchInput(msg)
		if valueChanged || handled {
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, cmd
	}

	// Handle pending prefix keys (Vim-style gg)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirm dialog
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			return m.deleteTask()
		}
		return m, nil
	}

	// Navigation
	if handled, cmd := handleListNavigation(key, &m.cursor, m.totalTasks, m.Height(), &m.cursorFlash); handled {
		return m, cmd
	}

	// Common task actions (t, Z, D, e, r, y/Y/ctrl+y, ?)
	if model, cmd, handled := handleCommonTaskAction(key, m); handled {
		return model, cmd
	}

	// Tab row focus switching
	switch key {
	case "(", ")":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = !m.subTabRowFocused
		}
		return m, nil
	case "[", "h":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			CycleTab(&m.subTab, total, -1)
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			m.tabs.Cycle(-1)
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	case "]", "l":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			CycleTab(&m.subTab, total, 1)
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			m.tabs.Cycle(1)
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	}

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, ",gdF", "mc", m.getTaskAtCursor() != nil); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	switch key {
	case "esc", "q":
		if RunEscCascade(m.escSteps()) {
			return m, nil
		}
		return m, func() tea.Msg {
			return ClosePriorityViewMsg{}
		}

	case "enter":
		if pt := m.getTaskAtCursor(); pt != nil {
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return m, nil

	// Toggle waiting on section
	case "tab":
		m.waitingOnCollapsed = !m.waitingOnCollapsed
		return m, nil

	// Priority changes
	case "1":
		return m.changeTaskPriority(1)
	case "2":
		return m.changeTaskPriority(2)
	case "3":
		return m.changeTaskPriority(3)
	case "4":
		return m.changeTaskPriority(4)
	case "5":
		return m.changeTaskPriority(5)

	// Suggestion actions
	case "s":
		// Start suggested task (or jump to it if already in progress)
		return m.startSuggestedTask()

	case "S":
		// Open score editor for task under cursor
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: pt.Task, Project: m.project}
			}
		}
		return m, nil

	case "x":
		// Dismiss current suggestion
		m.dismissSuggestion()
		return m, nil

	case "/":
		// Activate search mode
		return m, m.ActivateSearch()

	// Open all tasks view
	case "A":
		return m, func() tea.Msg {
			return ShowAllTasksMsg{}
		}
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *PriorityViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	viewSpecific := func(pfx, k string) (tea.Model, tea.Cmd, bool) {
		if pfx == "g" {
			switch k {
			case "t":
				if pt := m.getTaskAtCursor(); pt != nil {
					return m, func() tea.Msg {
						return PriorityViewSelectMsg{Task: pt.Task}
					}, true
				}
				return m, nil, true
			case "h":
				if pt := m.getTaskAtCursor(); pt != nil {
					project := m.project
					return m, func() tea.Msg {
						return OpenTaskActivityMsg{Task: pt.Task, Project: project}
					}, true
				}
				return m, nil, true
			}
		}
		return nil, nil, false
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, viewSpecific); handled {
		return model, cmd
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// prefixDispatchView implementation for PriorityViewModel.

func (m *PriorityViewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *PriorityViewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return PriorityViewSelectMsg{Task: task}
}

func (m *PriorityViewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handlePriorityFilterPrefix(key)
}

func (m *PriorityViewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handlePriorityFilterSubPrefix(prefix, key)
}
