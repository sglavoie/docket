package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for WeeklyReviewModel.

func (m *WeeklyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *WeeklyReviewModel) getPersister() Persister { return defaultPersister }
func (m *WeeklyReviewModel) getDataDir() string      { return m.config.DataDir }
func (m *WeeklyReviewModel) reloadData()             { m.loadData() }
func (m *WeeklyReviewModel) setErr(err error)        { m.err = err }
func (m *WeeklyReviewModel) setStatus(msg string)    { m.statusMessage = msg }

// prefixDispatchView implementation.

func (m *WeeklyReviewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *WeeklyReviewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return WeeklyReviewSelectTaskMsg{Task: task, Project: project}
}

func (m *WeeklyReviewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m, nil // WeeklyReview has no filter prefix
}

func (m *WeeklyReviewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m, nil // WeeklyReview has no filter sub-prefix
}
