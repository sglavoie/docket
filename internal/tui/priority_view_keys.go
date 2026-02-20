package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

func (m *PriorityViewModel) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (Vim-style gg)
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Handle confirm dialog
	if m.confirm.IsActive() {
		confirmed, handled := m.confirm.HandleKey(key)
		if handled {
			if confirmed {
				return m.deleteTask()
			}
			m.statusMessage = ""
			return m, nil
		}
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
			if m.subTab > 0 {
				m.subTab--
			} else {
				m.subTab = total - 1
			}
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(priorityTabNames) - 1
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	case "]", "l":
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			if m.subTab < total-1 {
				m.subTab++
			} else {
				m.subTab = 0
			}
			m.recalculateNavigation()
			m.clampCursor()
		} else {
			if m.activeTab < len(priorityTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.subTab = 0
			m.subTabRowFocused = false
			m.recalculateNavigation()
			m.clampCursor()
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
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

	// Navigation
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < m.totalTasks-1 {
			m.cursor++
		}
		return m, nil

	case ",":
		// Start prefix sequence for status operations
		m.pendingPrefix = ","
		return m, nil

	case "g":
		// Start prefix sequence for gg (Vim-style)
		m.pendingPrefix = "g"
		return m, nil

	case "d":
		// Start prefix sequence for dd (due date)
		m.pendingPrefix = "d"
		return m, nil

	case "m":
		// Start prefix sequence for section move
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "m"
		}
		return m, nil

	case "c":
		// Start prefix sequence for create linked task
		if m.getTaskAtCursor() != nil {
			m.pendingPrefix = "c"
		}
		return m, nil

	case "G":
		if m.totalTasks > 0 {
			m.cursor = m.totalTasks - 1
		}
		return m, nil

	case "ctrl+u":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil

	case "ctrl+d":
		m.cursor += 10
		if m.cursor >= m.totalTasks {
			m.cursor = m.totalTasks - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
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

	case "e":
		if pt := m.getTaskAtCursor(); pt != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: pt.Task}
			}
		}
		return m, nil

	case "y", "Y", "ctrl+y":
		var task *models.Task
		if pt := m.getTaskAtCursor(); pt != nil {
			task = pt.Task
		}
		if cmd, handled := clipboardCmd(key, task); handled {
			return m, cmd
		}
		return m, nil

	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewPriorityView,
				ViewName:   "Priority View",
				Items:      m.GetHelpItems(),
			}
		}

	case "r":
		// Refresh: reload project and clear dismissed suggestions
		_ = m.projectService.Load(m.project)
		m.dismissedIDs = make(map[string]bool)
		m.loadActionableTasks()
		m.clampCursor()
		return m, nil

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

	case "F":
		// Open filter prefix popup
		m.pendingPrefix = "F"
		return m, nil

	// Tags
	case "t":
		if pt := m.getTaskAtCursor(); pt != nil {
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: append([]string{}, task.Tags...)}
			}
		}
		return m, nil

	// Snooze
	case "Z":
		if pt := m.getTaskAtCursor(); pt != nil {
			project := m.project
			task := pt.Task
			return m, func() tea.Msg {
				return SetSnoozeDateMsg{Task: task, Project: project}
			}
		}
		return m, nil

	// Delete with confirmation
	case "D":
		if pt := m.getTaskAtCursor(); pt != nil {
			m.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", pt.Task.ShortID))
			m.statusMessage = m.confirm.Prompt()
		}
		return m, nil

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
