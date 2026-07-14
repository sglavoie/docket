package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"

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

// handleFilterPrefix handles the second key after pressing F (filter prefix).
func (m *ProjectFocusViewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fd, Fa, FS, F.).
func (m *ProjectFocusViewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
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

func (m *ProjectFocusViewModel) getRuntime() *config.RuntimeState  { return m.runtime }
func (m *ProjectFocusViewModel) getConfigManager() *config.Manager { return nil }
func (m *ProjectFocusViewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

// SetTaskItemCoordinator injects the application-layer single-task coordinator so
// the shell wires it from the shared showView/replaceView site (taskItemWireable).
func (m *ProjectFocusViewModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// workflowOpsView accessors: ProjectFocus does not embed ViewBundle.
func (m *ProjectFocusViewModel) taskItemCoordinator() TaskItemCoordinator { return m.taskItem }
func (m *ProjectFocusViewModel) workflowRules() []workflow.WorkflowRule {
	if m.runtime == nil {
		return nil
	}
	return m.runtime.Automation().GetEnabledWorkflowRules()
}

func (m *ProjectFocusViewModel) changeTaskPriority(key string) (tea.Model, tea.Cmd) {
	return m, wfChangePriority(m, key)
}
