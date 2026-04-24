package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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

func (m *DailyReviewModel) getPersister() Persister           { return defaultPersister }
func (m *DailyReviewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *DailyReviewModel) reloadData()                       { m.loadTasks() }
func (m *DailyReviewModel) setErr(err error)                  { m.err = err }
func (m *DailyReviewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }

// prefixDispatchView implementation.

func (m *DailyReviewModel) selectMsg(task *models.Task, project *models.Project) tea.Msg {
	return DailyReviewSelectMsg{Task: task, Project: project}
}

func (m *DailyReviewModel) handleFilterPrefix(key string) (tea.Model, tea.Cmd) {
	return m.handleDailyReviewFilterPrefix(key)
}

func (m *DailyReviewModel) handleFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	return m.handleDailyReviewFilterSubPrefix(prefix, key)
}
