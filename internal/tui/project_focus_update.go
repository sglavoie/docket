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
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
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
