package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// trackMutation increments the count for a mutation category.
func (m *WeeklyReviewModel) trackMutation(category string) {
	m.mutationCounts[category]++
}

// taskOpsView implementation for WeeklyReviewModel.

func (m *WeeklyReviewModel) getTaskRef() (*models.Task, *models.Project) {
	dt := m.getTaskAtCursor()
	if dt == nil {
		return nil, nil
	}
	return dt.Task, dt.Project
}

func (m *WeeklyReviewModel) getPersister() Persister           { return defaultPersister }
func (m *WeeklyReviewModel) getConfig() *config.Config         { return m.config }
func (m *WeeklyReviewModel) getConfigManager() *config.Manager { return m.configManager }
func (m *WeeklyReviewModel) reloadData()                       { m.loadData() }
func (m *WeeklyReviewModel) setErr(err error)                  { m.err = err }
func (m *WeeklyReviewModel) setStatus(msg string)              { m.InteractiveViewState.setStatus(msg) }
