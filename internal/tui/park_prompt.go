package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

type pendingParkNavigation struct {
	source      TaskIdentity
	continueNav func() tea.Cmd
}

func (m *AppModel) maybePromptPark(target TaskIdentity, continueNav func() tea.Cmd) (tea.Cmd, bool) {
	mutation, ok := lastSessionMutation()
	if !ok || mutation.TaskID == "" {
		return continueNav(), false
	}
	source := TaskIdentity{ProjectID: mutation.ProjectID, TaskID: mutation.TaskID}
	if target.Valid() && source == target {
		return continueNav(), false
	}
	if wasParkPrompted(source) {
		return continueNav(), false
	}

	project := m.projectForSessionMutation(mutation)
	if project == nil {
		return continueNav(), false
	}
	task := project.GetTaskByID(mutation.TaskID)
	if task == nil || task.IsComplete() || taskParkedSinceMutation(task, mutation) {
		return continueNav(), false
	}

	m.pendingParkNav = &pendingParkNavigation{
		source:      source,
		continueNav: continueNav,
	}
	parkInput := NewParkInputModel(task, project)
	parkInput.promptMode = true
	m.showView(ViewParkInput, parkInput)
	return nil, true
}

func (m *AppModel) projectForSessionMutation(mutation sessionTaskMutation) *models.Project {
	if m.currentProject != nil && m.currentProject.ID == mutation.ProjectID {
		return m.currentProject
	}
	if m.runtime == nil {
		return nil
	}
	return m.projectByID(mutation.ProjectID)
}

func taskParkedSinceMutation(task *models.Task, mutation sessionTaskMutation) bool {
	if task.ParkedAt == nil {
		return task.IsParked()
	}
	return !task.ParkedAt.Before(mutation.LastMutated)
}

func (m *AppModel) finishPendingParkPrompt(msg ParkNoteDoneMsg) (bool, tea.Cmd) {
	pending := m.pendingParkNav
	if pending == nil {
		return false, nil
	}

	identity := pending.source
	if msg.Task != nil && msg.Project != nil {
		identity = taskIdentityFor(msg.Task, msg.Project)
	}
	markParkPrompted(identity)

	if msg.Skipped || msg.Task == nil || msg.Project == nil {
		return true, m.runPendingParkContinuation()
	}

	res, err := m.applyParkMutation(msg)
	if err != nil {
		m.err = fmt.Errorf("park prompt save failed: %w", err)
		return true, nil
	}
	m.reloadProjectByID(res.ProjectID)

	return true, m.runPendingParkContinuation()
}

func (m *AppModel) runPendingParkContinuation() tea.Cmd {
	pending := m.pendingParkNav
	m.pendingParkNav = nil
	popCmd := m.popView()
	if pending == nil || pending.continueNav == nil {
		return popCmd
	}
	return tea.Batch(popCmd, pending.continueNav())
}
