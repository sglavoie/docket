package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"

	tea "charm.land/bubbletea/v2"
)

func (m *ProjectFocusViewModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectFocusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(
		msg,
		m.SetSize,
		func() { _ = defaultPersister.Load(m.project); m.loadTasks() },
		m.clampCursor,
		m.setStatus,
	); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg handles keyboard input.
func (m *ProjectFocusViewModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys (which-key style)
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	// Clear previous error on any key
	m.ClearError()
	m.clearStatus()

	// Activate prefix mode
	if prefix, ok := activateSecondaryPrefix(key, ",dg", "", false); ok {
		m.pendingPrefix = prefix
		return m, nil
	}

	// Sub-tab navigation
	switch key {
	case "shift+down":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = true
		}
		return m, nil
	case "shift+up":
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = false
		}
		return m, nil
	case "tab", "shift+right":
		if len(m.subTabNames) > 1 {
			CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
			m.clampCursor()
			return m, nil
		}
	case "shift+tab", "shift+left":
		if len(m.subTabNames) > 1 {
			CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
			m.clampCursor()
			return m, nil
		}
	}

	tasks := m.filteredTasks()

	if handled, cmd := handleListNavigation(key, &m.cursor, len(tasks), m.ContentHeight(), &m.cursorFlash); handled {
		return m, cmd
	}

	switch key {
	case "esc":
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	case "enter":
		// Close and return to project tree (task is already in this project)
		return m, func() tea.Msg {
			return CloseProjectFocusViewMsg{}
		}

	// Toggle active flag (remove from focus)
	case "*":
		return m.toggleActive()

	// Priority changes
	case "1", "2", "3", "4", "5":
		return m.changeTaskPriority(key)

	// Edit task
	case "e":
		if task := m.getTaskAtCursor(); task != nil {
			return m, func() tea.Msg {
				return EditTaskMsg{Task: task}
			}
		}
		return m, nil

	// Refresh
	case "r":
		_ = defaultPersister.Load(m.project)
		m.loadTasks()
		m.clampCursor()
		m.setStatus("Refreshed")
		return m, nil

	// Help
	case "?":
		return m, helpCmd(ViewProjectFocusView, "Project Focus View", m.GetHelpItems())
	}

	return m, nil
}

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *ProjectFocusViewModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	if model, cmd, handled := dispatchCommonPrefix(prefix, key, m, nil); handled {
		return model, cmd
	}

	// Invalid prefix sequence - ignore
	return m, nil
}

// prefixDispatchView implementation for ProjectFocusViewModel.

func (m *ProjectFocusViewModel) selectMsg(_ *models.Task, _ *models.Project) tea.Msg {
	return CloseProjectFocusViewMsg{}
}

func (m *ProjectFocusViewModel) handleFilterPrefix(_ string) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *ProjectFocusViewModel) handleFilterSubPrefix(_, _ string) (tea.Model, tea.Cmd) {
	return m, nil
}

// taskOpsView implementation for ProjectFocusViewModel.

func (m *ProjectFocusViewModel) getTaskRef() (*models.Task, *models.Project) {
	task := m.getTaskAtCursor()
	if task == nil {
		return nil, nil
	}
	return task, m.project
}

func (m *ProjectFocusViewModel) getPersister() Persister           { return defaultPersister }
func (m *ProjectFocusViewModel) getConfig() *config.Config         { return m.cfg }
func (m *ProjectFocusViewModel) getConfigManager() *config.Manager { return nil }
func (m *ProjectFocusViewModel) reloadData()                       { m.loadTasks() }
func (m *ProjectFocusViewModel) setErr(err error)                  { m.SetError(err) }
func (m *ProjectFocusViewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

func (m *ProjectFocusViewModel) toggleActive() (tea.Model, tea.Cmd) {
	if task := m.getTaskAtCursor(); task != nil {
		return taskOpsRemoveFromFocus(m, task.ShortID)
	}
	return m, nil
}

func (m *ProjectFocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return taskOpsChangePriority(m, key)
}
