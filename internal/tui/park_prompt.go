package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

type pendingParkNavigation struct {
	sourceTaskID string
	continueNav  func() tea.Cmd
}

func (m *AppModel) maybePromptPark(targetTaskID string, continueNav func() tea.Cmd) (tea.Cmd, bool) {
	mutation, ok := lastSessionMutation()
	if !ok || mutation.TaskID == "" {
		return continueNav(), false
	}
	if targetTaskID != "" && mutation.TaskID == targetTaskID {
		return continueNav(), false
	}
	if wasParkPrompted(mutation.TaskID) {
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
		sourceTaskID: mutation.TaskID,
		continueNav:  continueNav,
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

	taskID := pending.sourceTaskID
	if msg.Task != nil {
		taskID = msg.Task.ID
	}
	markParkPrompted(taskID)

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
