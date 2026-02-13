package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView implementation for DailyReviewModel.

func (m *DailyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *DailyReviewModel) getPersister() Persister { return defaultPersister }
func (m *DailyReviewModel) reloadData()             { m.loadTasks() }
func (m *DailyReviewModel) setErr(err error)        { m.err = err }
func (m *DailyReviewModel) setStatus(msg string)    { m.statusMessage = msg }

// prefixDispatchView implementation.

func (m *DailyReviewModel) getTaskAndProject() (*models.Task, *models.Project) {
	return m.getTaskRef()
}

func (m *DailyReviewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return DailyReviewSelectMsg{Task: task, Project: project}
}

func (m *DailyReviewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleDailyReviewFilterPrefix(key)
}

func (m *DailyReviewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleDailyReviewFilterSubPrefix(prefix, key)
}
